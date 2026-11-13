<?php

namespace Riven\Scanner;

use Illuminate\Http\Resources\Json\AnonymousResourceCollection;
use Illuminate\Http\Resources\Json\JsonResource;
use Illuminate\Http\Resources\Json\ResourceCollection;
use Illuminate\Routing\Route;
use Illuminate\Support\Str;
use PhpParser\Error as ParseError;
use PhpParser\Node;
use PhpParser\Node\Expr;
use PhpParser\Node\Name;
use PhpParser\Node\Stmt;
use PhpParser\NodeFinder;
use PhpParser\NodeTraverser;
use PhpParser\NodeVisitor;
use PhpParser\NodeVisitor\NameResolver;
use PhpParser\NodeVisitorAbstract;
use PhpParser\Parser;
use PhpParser\ParserFactory;
use ReflectionClass;
use ReflectionFunctionAbstract;
use ReflectionNamedType;

/**
 * Best-effort, static detection of the API Resource an action returns.
 * Anything it can't be sure about is null, never a guess.
 */
class ResourceDetector
{
    private const UNKNOWN = ['resource' => null, 'collection' => null];

    private Parser $parser;

    /** @var array<string, Node[]|null> Parsed (name-resolved) files, null if unparseable */
    private array $files = [];

    public function __construct(private ActionReflector $actions = new ActionReflector)
    {
        $this->parser = (new ParserFactory)->createForHostVersion();
    }

    /**
     * @return array{resource: ?string, collection: ?bool}
     */
    public function detect(Route $route): array
    {
        $function = $this->actions->reflect($route);

        if ($function === null) {
            return self::UNKNOWN;
        }

        return $this->fromReturnType($function) ?? $this->fromReturnStatements($function) ?? self::UNKNOWN;
    }

    /**
     * A declared return type is the strongest signal. AnonymousResourceCollection
     * (what X::collection() returns) doesn't name X, so it falls through to parsing.
     */
    private function fromReturnType(ReflectionFunctionAbstract $function): ?array
    {
        $type = $function->getReturnType();

        if (! $type instanceof ReflectionNamedType || $type->isBuiltin()) {
            return null;
        }

        return $this->describeClass($type->getName());
    }

    /**
     * A named collection (UserCollection) reports the resource it collects, if declared.
     */
    private function describeClass(string $class): ?array
    {
        if (! is_subclass_of($class, JsonResource::class)) {
            return null;
        }

        if (! is_subclass_of($class, ResourceCollection::class)) {
            return ['resource' => $class, 'collection' => false];
        }

        // The generic collection X::collection() returns doesn't name X.
        if ($class === AnonymousResourceCollection::class) {
            return null;
        }

        return ['resource' => $this->collectedResource($class) ?? $class, 'collection' => true];
    }

    /**
     * Mirrors ResourceCollection::collects(): the $collects property, else the
     * naming convention UserCollection → User / UserResource.
     */
    private function collectedResource(string $collection): ?string
    {
        $collects = (new ReflectionClass($collection))->getDefaultProperties()['collects'] ?? null;

        if ($collects !== null) {
            return $collects;
        }

        if (! str_ends_with($collection, 'Collection')) {
            return null;
        }

        foreach ([Str::replaceLast('Collection', '', $collection), Str::replaceLast('Collection', 'Resource', $collection)] as $candidate) {
            if (is_subclass_of($candidate, JsonResource::class)) {
                return $candidate;
            }
        }

        return null;
    }

    private function fromReturnStatements(ReflectionFunctionAbstract $function): ?array
    {
        $node = $this->findFunctionNode($function);

        if ($node === null) {
            return null;
        }

        $results = array_map(fn (?Expr $expr) => $expr ? $this->describeExpr($expr) : null, $this->returnedExprs($node));

        // Every return must agree; one unknown or conflicting return makes the whole answer unknown.
        if ($results === [] || in_array(null, $results, true) || count(array_unique($results, SORT_REGULAR)) > 1) {
            return null;
        }

        return $results[0];
    }

    /**
     * new X(...), X::make(...), X::collection(...), looking through chained calls
     * such as ->additional([...]) or ->response()->setStatusCode(201).
     */
    private function describeExpr(Expr $expr): ?array
    {
        while ($expr instanceof Expr\MethodCall || $expr instanceof Expr\NullsafeMethodCall) {
            $expr = $expr->var;
        }

        if ($expr instanceof Expr\New_ && $expr->class instanceof Name) {
            return $this->describeClass($expr->class->toString());
        }

        if ($expr instanceof Expr\StaticCall && $expr->class instanceof Name && $expr->name instanceof Node\Identifier) {
            $class = $expr->class->toString();

            if (! is_subclass_of($class, JsonResource::class)) {
                return null;
            }

            return match ($expr->name->toLowerString()) {
                'make' => $this->describeClass($class),
                'collection' => ['resource' => $class, 'collection' => true],
                default => null,
            };
        }

        return null;
    }

    /**
     * The AST node for the reflected method or closure, matched by start line.
     */
    private function findFunctionNode(ReflectionFunctionAbstract $function): ?Node\FunctionLike
    {
        $file = $function->getFileName();
        $ast = $file ? $this->parse($file) : null;

        if ($ast === null) {
            return null;
        }

        return (new NodeFinder)->findFirst($ast, fn (Node $node) => $node instanceof Node\FunctionLike
            && ! $node instanceof Stmt\Function_
            && $node->getStartLine() === $function->getStartLine()
            && ($function->isClosure() xor $node instanceof Stmt\ClassMethod)
            && (! $node instanceof Stmt\ClassMethod || $node->name->toString() === $function->getName()));
    }

    /**
     * Expressions returned by this function itself, not by closures nested inside it.
     *
     * @return array<int, ?Expr>
     */
    private function returnedExprs(Node\FunctionLike $function): array
    {
        if ($function instanceof Expr\ArrowFunction) {
            return [$function->expr];
        }

        $visitor = new class extends NodeVisitorAbstract
        {
            /** @var array<int, ?Expr> */
            public array $exprs = [];

            public function enterNode(Node $node)
            {
                if ($node instanceof Node\FunctionLike || $node instanceof Stmt\ClassLike) {
                    return NodeVisitor::DONT_TRAVERSE_CHILDREN;
                }

                if ($node instanceof Stmt\Return_) {
                    $this->exprs[] = $node->expr;
                }

                return null;
            }
        };

        $traverser = new NodeTraverser($visitor);
        $traverser->traverse($function->getStmts() ?? []);

        return $visitor->exprs;
    }

    /**
     * @return Node[]|null
     */
    private function parse(string $file): ?array
    {
        if (! array_key_exists($file, $this->files)) {
            try {
                // NameResolver turns short names into FQCNs using the file's `use` imports.
                $this->files[$file] = (new NodeTraverser(new NameResolver))
                    ->traverse($this->parser->parse((string) file_get_contents($file)) ?? []);
            } catch (ParseError) {
                $this->files[$file] = null;
            }
        }

        return $this->files[$file];
    }
}
