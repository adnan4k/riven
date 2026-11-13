<?php

namespace Riven\Faking;

use BackedEnum;
use Closure;
use Faker\Generator;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Validator;
use Illuminate\Support\Str;
use Illuminate\Validation\Rules\Enum;
use Illuminate\Validation\Rules\Password;
use ReflectionProperty;
use Stringable;
use Throwable;

/**
 * Builds request bodies that pass a FormRequest's rules, without AI:
 * Faker values shaped by the rules (types, sizes, in/enum, regex…), real
 * database values for exists:, fresh values for unique:, then a check with
 * Laravel's own Validator. It only reads from the database.
 */
class BodyFaker
{
    private const ATTEMPTS = 3;

    /** @var list<string> */
    private array $warnings = [];

    public function __construct(private Generator $faker) {}

    /**
     * @param  array<string, list<mixed>>  $rules  RulesExtractor::raw() output
     * @return array{body: array<string, mixed>, errors: array<string, list<string>>, warnings: list<string>}
     */
    public function make(array $rules): array
    {
        $this->warnings = [];
        $best = null;

        for ($i = 0; $i < self::ATTEMPTS; $i++) {
            $body = $this->build($rules);
            $errors = $this->validate($body, $rules);

            if ($best === null || count($errors) < count($best['errors'])) {
                $best = ['body' => $body, 'errors' => $errors];
            }
            if ($errors === []) {
                break;
            }
        }

        return [...$best, 'warnings' => array_values(array_unique($this->warnings))];
    }

    /**
     * @param  array<string, list<mixed>>  $rules
     * @return array<string, mixed>
     */
    private function build(array $rules): array
    {
        // Field tree: "items.*.id" is items → * → id.
        $tree = [];
        foreach ($rules as $field => $list) {
            $node = &$tree;
            foreach (explode('.', (string) $field) as $segment) {
                $node['children'][$segment] ??= [];
                $node = &$node['children'][$segment];
            }
            $node['field'] = (string) $field;
            $node['spec'] = $this->spec($list);
            unset($node);
        }

        $body = $this->object($tree['children'] ?? []);

        // confirmed / same: copy after everything exists.
        foreach ($rules as $field => $list) {
            $spec = $this->spec($list);
            if ($spec['confirmed'] && ! str_contains((string) $field, '*') && data_get($body, $field) !== null) {
                data_set($body, $field.'_confirmation', data_get($body, $field));
            }
            if ($spec['same'] !== null && ! str_contains((string) $field, '*')) {
                data_set($body, $field, data_get($body, $spec['same']));
            }
        }

        return $body;
    }

    /**
     * @param  array<string, array<string, mixed>>  $children
     * @return array<string, mixed>
     */
    private function object(array $children): array
    {
        $out = [];
        foreach ($children as $name => $node) {
            if ($name === '*') {
                continue;
            }
            if (($node['spec']['prohibited'] ?? false)) {
                continue;
            }
            $value = $this->node($node, (string) $name);
            if ($value !== null || ($node['spec']['nullable'] ?? false)) {
                $out[$name] = $value;
            }
        }

        return $out;
    }

    private function node(array $node, string $name): mixed
    {
        $spec = $node['spec'] ?? $this->spec([]);
        $children = $node['children'] ?? [];

        if (isset($children['*'])) {
            // A list: between its min/size and a small default, so bodies stay readable.
            $count = $spec['size'] ?? max((int) ($spec['min'] ?? 1), min((int) ($spec['max'] ?? 2), 2));

            return array_map(fn () => $this->node($children['*'], Str::singular($name)), range(1, max(1, (int) $count)));
        }
        if ($children !== []) {
            return $this->object($children);
        }

        return $this->leaf($spec, $name, $node['field'] ?? $name);
    }

    /**
     * Turns one field's rules into what generation needs.
     *
     * @param  list<mixed>  $rules
     * @return array<string, mixed>
     */
    private function spec(array $rules): array
    {
        $spec = [
            'type' => null, 'format' => null, 'min' => null, 'max' => null, 'size' => null,
            'in' => null, 'not_in' => [], 'regex' => null, 'digits' => null, 'digits_min' => null, 'digits_max' => null,
            'nullable' => false, 'confirmed' => false, 'same' => null, 'prohibited' => false,
            'exists' => null, 'unique' => null, 'starts' => null, 'ends' => null, 'case' => null,
            'after' => null, 'before' => null, 'accepted' => null, 'file' => false, 'password_min' => null,
        ];

        foreach ($rules as $rule) {
            foreach ($this->ruleStrings($rule, $spec) as $string) {
                [$name, $arg] = array_pad(explode(':', $string, 2), 2, null);
                $args = $arg === null ? [] : str_getcsv($arg, ',', '"', '\\');

                match (strtolower($name)) {
                    'string', 'integer', 'int', 'numeric', 'decimal', 'boolean', 'bool', 'array', 'list', 'json' => $spec['type'] = strtolower($name),
                    'email', 'url', 'active_url', 'uuid', 'ulid', 'ip', 'ipv4', 'ipv6', 'mac_address', 'timezone', 'date', 'hex_color', 'alpha', 'alpha_num', 'alpha_dash', 'ascii' => $spec['format'] = strtolower($name),
                    'date_format' => $spec['format'] = 'date_format:'.$arg,
                    'min' => $spec['min'] = (float) ($args[0] ?? 0),
                    'max' => $spec['max'] = (float) ($args[0] ?? 0),
                    'size' => $spec['size'] = (float) ($args[0] ?? 0),
                    'between' => [$spec['min'], $spec['max']] = [(float) ($args[0] ?? 0), (float) ($args[1] ?? 0)],
                    'digits' => $spec['digits'] = (int) ($args[0] ?? 1),
                    'digits_between' => [$spec['digits_min'], $spec['digits_max']] = [(int) ($args[0] ?? 1), (int) ($args[1] ?? 1)],
                    'in' => $spec['in'] = $args,
                    'not_in' => $spec['not_in'] = $args,
                    'regex' => $spec['regex'] = $arg,
                    'nullable' => $spec['nullable'] = true,
                    'confirmed' => $spec['confirmed'] = true,
                    'same' => $spec['same'] = $args[0] ?? null,
                    'prohibited' => $spec['prohibited'] = true,
                    'exists' => $spec['exists'] = $args,
                    'unique' => $spec['unique'] = $args,
                    'starts_with' => $spec['starts'] = $args[0] ?? null,
                    'ends_with' => $spec['ends'] = $args[0] ?? null,
                    'lowercase', 'uppercase' => $spec['case'] = strtolower($name),
                    'after', 'after_or_equal' => $spec['after'] = $args[0] ?? 'today',
                    'before', 'before_or_equal' => $spec['before'] = $args[0] ?? 'today',
                    'accepted' => $spec['accepted'] = true,
                    'declined' => $spec['accepted'] = false,
                    'file', 'image', 'mimes', 'mimetypes', 'extensions', 'dimensions' => $spec['file'] = true,
                    default => null, // required, sometimes, bail, conditional rules…: don't change the value
                };
            }
        }

        return $spec;
    }

    /**
     * String forms of one rule; rule objects that carry more (enum cases,
     * password length) write it into $spec directly.
     *
     * @return list<string>
     */
    private function ruleStrings(mixed $rule, array &$spec): array
    {
        if (is_string($rule)) {
            return [$rule];
        }
        if ($rule instanceof Closure) {
            return [];
        }
        if ($rule instanceof Enum) {
            $type = $this->property($rule, 'type');
            if (is_string($type) && enum_exists($type) && is_subclass_of($type, BackedEnum::class)) {
                $spec['in'] = array_map(fn (BackedEnum $case) => $case->value, $type::cases());
            }

            return [];
        }
        if ($rule instanceof Password) {
            $spec['password_min'] = (int) ($this->property($rule, 'min') ?? 8);

            return [];
        }
        if ($rule instanceof Stringable) {
            try {
                return explode('|', (string) $rule); // Rule::in(), Rule::exists(), fluent string/numeric rules…
            } catch (Throwable) {
                return [];
            }
        }

        return []; // custom rule objects: nothing to read; the Validator still checks them
    }

    private function leaf(array $spec, string $name, string $field): mixed
    {
        if ($spec['file']) {
            $this->warnings[] = "{$field}: file uploads can't be generated in a JSON body.";

            return null;
        }
        if ($spec['accepted'] !== null) {
            return $spec['accepted'];
        }
        if ($spec['in'] !== null && $spec['in'] !== []) {
            return $this->typed($this->faker->randomElement($spec['in']), $spec['type']);
        }
        if ($spec['exists'] !== null) {
            return $this->existing($spec['exists'], $name, $field);
        }

        for ($try = 0; $try < 10; $try++) {
            $value = $this->generate($spec, $name);
            if (in_array((string) $value, $spec['not_in'], true)) {
                continue;
            }
            if ($spec['unique'] !== null && is_scalar($value) && $this->taken($spec['unique'], $name, $value)) {
                continue;
            }

            return $value;
        }

        $this->warnings[] = "{$field}: couldn't find a value that isn't taken yet.";

        return $value;
    }

    private function generate(array $spec, string $name): mixed
    {
        $n = strtolower($name);
        $type = $spec['type'];
        $format = $spec['format'];

        if ($type === 'boolean' || $type === 'bool') {
            return $this->faker->boolean();
        }
        if ($spec['digits'] !== null || $spec['digits_min'] !== null) {
            $len = $spec['digits'] ?? $this->faker->numberBetween($spec['digits_min'], $spec['digits_max']);

            return $type === 'string' ? $this->faker->numerify(str_repeat('#', $len)) : (int) ($this->faker->numberBetween(1, 9).$this->faker->numerify(str_repeat('#', max(0, $len - 1))));
        }
        if (in_array($type, ['integer', 'int', 'numeric', 'decimal'], true)) {
            return $this->number($spec, $n, $type);
        }
        if ($type === 'array' || $type === 'list') {
            $count = (int) ($spec['size'] ?? max($spec['min'] ?? 1, 1));

            return array_map(fn () => $this->faker->word(), range(1, $count));
        }
        if ($type === 'json') {
            return json_encode(['key' => $this->faker->word()]);
        }
        if ($spec['password_min'] !== null || str_contains($n, 'password')) {
            $min = max($spec['password_min'] ?? 8, (int) ($spec['min'] ?? 0), 12);

            return 'Aa1!'.Str::random($min - 4); // upper, lower, number and symbol: passes Password::defaults()
        }
        if ($format !== null && ($v = $this->formatted($format, $spec)) !== null) {
            return $this->shape($v, $spec);
        }
        if ($spec['regex'] !== null && ($v = $this->fromRegex($spec['regex'])) !== null) {
            return $v;
        }
        if ($spec['after'] !== null || $spec['before'] !== null) {
            return $this->date($spec)->format('Y-m-d');
        }

        return $this->shape($this->byName($n), $spec);
    }

    private function number(array $spec, string $name, string $type): int|float
    {
        [$lo, $hi] = match (true) {
            str_contains($name, 'age') => [18, 80],
            (bool) preg_match('/qty|quantity|count/', $name) => [1, 5],
            (bool) preg_match('/price|amount|total|cost/', $name) => [1, 500],
            (bool) preg_match('/year/', $name) => [2000, 2030],
            default => [1, 1000],
        };
        $min = $spec['size'] ?? $spec['min'] ?? $lo;
        $max = $spec['size'] ?? $spec['max'] ?? max($hi, $min);

        return $type === 'integer' || $type === 'int'
            ? $this->faker->numberBetween((int) ceil($min), (int) floor(max($min, $max)))
            : $this->faker->randomFloat(2, $min, max($min, $max));
    }

    private function formatted(string $format, array $spec): ?string
    {
        if (str_starts_with($format, 'date_format:')) {
            return $this->date($spec)->format(substr($format, strlen('date_format:')));
        }

        return match ($format) {
            'email' => $this->faker->unique()->safeEmail(),
            'url', 'active_url' => 'https://example.com/'.$this->faker->slug(2),
            'uuid' => $this->faker->uuid(),
            'ulid' => (string) Str::ulid(),
            'ip', 'ipv4' => $this->faker->ipv4(),
            'ipv6' => $this->faker->ipv6(),
            'mac_address' => $this->faker->macAddress(),
            'timezone' => $this->faker->timezone(),
            'date' => $this->date($spec)->format('Y-m-d'),
            'hex_color' => $this->faker->hexColor(),
            'alpha' => $this->faker->lexify(str_repeat('?', 8)),
            'alpha_num' => $this->faker->bothify(str_repeat('?#', 4)),
            'alpha_dash' => $this->faker->slug(2),
            'ascii' => $this->faker->word(),
            default => null,
        };
    }

    private function date(array $spec): \DateTimeInterface
    {
        $after = $spec['after'] !== null ? strtotime((string) $spec['after']) : false;
        $before = $spec['before'] !== null ? strtotime((string) $spec['before']) : false;

        return match (true) {
            $after !== false && $before !== false => $this->faker->dateTimeBetween('@'.($after + 86400), '@'.max($after + 86400, $before - 86400)),
            $after !== false => $this->faker->dateTimeBetween('@'.($after + 86400), '@'.($after + 86400 * 60)),
            $before !== false => $this->faker->dateTimeBetween('@'.($before - 86400 * 365), '@'.($before - 86400)),
            default => $this->faker->dateTimeBetween('-1 year', '+1 year'),
        };
    }

    private function fromRegex(string $regex): ?string
    {
        $delim = $regex[0] ?? '/';
        $end = strrpos($regex, $delim);
        $pattern = $end > 0 ? substr($regex, 1, $end - 1) : $regex;

        try {
            $value = $this->faker->regexify($pattern);

            return @preg_match($regex, $value) === 1 ? $value : null;
        } catch (Throwable) {
            return null;
        }
    }

    // Field-name hints for strings with no format rule.
    private function byName(string $n): string
    {
        return match (true) {
            str_contains($n, 'email') => $this->faker->unique()->safeEmail(),
            str_contains($n, 'first_name') => $this->faker->firstName(),
            str_contains($n, 'last_name') => $this->faker->lastName(),
            (bool) preg_match('/user_?name|login|handle/', $n) => $this->faker->userName(),
            (bool) preg_match('/company|organi[sz]ation/', $n) => $this->faker->company(),
            str_contains($n, 'name') => $this->faker->name(),
            str_contains($n, 'title') || str_contains($n, 'subject') => rtrim($this->faker->sentence(4), '.'),
            str_contains($n, 'slug') => $this->faker->slug(3),
            (bool) preg_match('/phone|mobile|tel/', $n) => $this->faker->e164PhoneNumber(),
            (bool) preg_match('/address|street/', $n) => $this->faker->streetAddress(),
            str_contains($n, 'city') => $this->faker->city(),
            str_contains($n, 'state') || str_contains($n, 'region') => $this->faker->state(),
            str_contains($n, 'country') => $this->faker->countryCode(),
            (bool) preg_match('/zip|postal|postcode/', $n) => $this->faker->postcode(),
            (bool) preg_match('/url|website|link/', $n) => 'https://example.com/'.$this->faker->slug(2),
            (bool) preg_match('/description|body|content|bio|notes?|message|comment|summary/', $n) => $this->faker->paragraph(2),
            (bool) preg_match('/colou?r/', $n) => $this->faker->safeColorName(),
            (bool) preg_match('/coupon|code|sku|reference/', $n) => strtoupper($this->faker->bothify('???###')),
            str_contains($n, 'currency') => $this->faker->currencyCode(),
            (bool) preg_match('/locale|lang/', $n) => $this->faker->languageCode(),
            str_contains($n, 'token') => Str::random(32),
            default => $this->faker->words(2, true),
        };
    }

    // Applies prefix/suffix, case and string length limits.
    private function shape(string $value, array $spec): string
    {
        if ($spec['starts'] !== null && ! str_starts_with($value, $spec['starts'])) {
            $value = $spec['starts'].$value;
        }
        if ($spec['ends'] !== null && ! str_ends_with($value, $spec['ends'])) {
            $value .= $spec['ends'];
        }
        if ($spec['case'] === 'lowercase') {
            $value = mb_strtolower($value);
        } elseif ($spec['case'] === 'uppercase') {
            $value = mb_strtoupper($value);
        }

        $min = (int) ($spec['size'] ?? $spec['min'] ?? 0);
        $max = $spec['size'] ?? $spec['max'];
        while (mb_strlen($value) < $min) {
            $value .= ' '.$this->faker->word();
        }
        if ($max !== null && mb_strlen($value) > $max) {
            $value = rtrim(mb_substr($value, 0, (int) $max));
            while (mb_strlen($value) < $min) {
                $value .= 'x';
            }
        }

        return $value;
    }

    private function typed(mixed $value, ?string $type): mixed
    {
        return match (true) {
            in_array($type, ['integer', 'int'], true) && is_numeric($value) => (int) $value,
            in_array($type, ['numeric', 'decimal'], true) && is_numeric($value) => $value + 0,
            default => $value,
        };
    }

    /**
     * A real value for exists:table,column (column defaults to the field name).
     */
    private function existing(array $args, string $name, string $field): mixed
    {
        [$table, $column] = $this->tableColumn($args, $name);

        try {
            $value = DB::table($table)->inRandomOrder()->value($column);
        } catch (Throwable $e) {
            $this->warnings[] = "{$field}: couldn't read {$table}.{$column}: ".Str::limit($e->getMessage(), 120);

            return null;
        }

        if ($value === null) {
            $this->warnings[] = "{$field}: {$table} has no rows, so exists:{$table},{$column} can't be satisfied. Seed it first.";
        }

        return $value;
    }

    private function taken(array $args, string $name, mixed $value): bool
    {
        [$table, $column] = $this->tableColumn($args, $name);

        try {
            return DB::table($table)->where($column, $value)->exists();
        } catch (Throwable) {
            return false; // can't check: let the Validator report it
        }
    }

    /**
     * exists:table,column / unique:table,column, where table may be a model
     * class or "connection.table".
     *
     * @return array{0: string, 1: string}
     */
    private function tableColumn(array $args, string $name): array
    {
        $table = (string) ($args[0] ?? '');
        if (class_exists($table) && method_exists($table, 'getTable')) {
            $table = (new $table)->getTable();
        }
        $column = (string) ($args[1] ?? '');

        return [$table, $column === '' || $column === 'NULL' ? $name : $column];
    }

    /**
     * Field => messages, from Laravel's Validator with the real rules.
     *
     * @return array<string, list<string>>
     */
    private function validate(array $body, array $rules): array
    {
        try {
            return Validator::make($body, $rules)->errors()->toArray();
        } catch (Throwable $e) {
            $this->warnings[] = "Couldn't run the validator here: ".Str::limit($e->getMessage(), 160);

            return [];
        }
    }

    private function property(object $object, string $name): mixed
    {
        try {
            return (new ReflectionProperty($object, $name))->getValue($object);
        } catch (Throwable) {
            return null;
        }
    }
}
