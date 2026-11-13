// Package rules maps Laravel validation rules (as exported in the manifest:
// field => ["required", "string", "max:255"]) to JSON Schema (2020-12, the
// dialect OpenAPI 3.1 uses).
//
// It only maps what a rule says for sure. Everything else stays visible:
// each property keeps its raw rules in "x-laravel-rules", and rules with no
// schema equivalent are reported in Result.Unmapped.
package rules

import (
	"encoding/csv"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Schema is a JSON Schema object, built as a map so it serialises exactly.
type Schema = map[string]any

type Result struct {
	Schema Schema
	// Multipart is true when a field is a file upload, so the body must be
	// multipart/form-data rather than JSON.
	Multipart bool
	// Unmapped lists "field: rule" pairs with no JSON Schema equivalent
	// (exists:, unique:, rule objects, closures…), for docs or the AI pass.
	Unmapped []string
}

// ToSchema builds an object schema for a whole FormRequest.
func ToSchema(rules map[string][]string) Result {
	root := newNode()
	res := Result{}

	for _, field := range slices.Sorted(maps.Keys(rules)) {
		n := root.child(strings.Split(field, "."))
		n.rules = rules[field]
		n.field = field
	}

	res.Schema = root.schema(&res)
	return res
}

// node is one level of the dotted field tree: "items.*.id" is
// root → items → * → id.
type node struct {
	field    string
	rules    []string
	children map[string]*node
	order    []string
}

func newNode() *node { return &node{children: map[string]*node{}} }

func (n *node) child(path []string) *node {
	if len(path) == 0 {
		return n
	}
	c, ok := n.children[path[0]]
	if !ok {
		c = newNode()
		n.children[path[0]] = c
		n.order = append(n.order, path[0])
	}
	return c.child(path[1:])
}

func (n *node) schema(res *Result) Schema {
	s := Schema{}
	parsed := parse(n.rules)
	typ := parsed.typ

	// Children decide the shape when no type rule does: "*" means a list,
	// named keys mean an object.
	if star, ok := n.children["*"]; ok {
		if typ == "" || typ == "object" {
			typ = "array"
		}
		s["items"] = star.schema(res)
	}
	if named := n.namedChildren(); len(named) > 0 {
		// Named keys make an object, also for `array` rules without "*" (an
		// associative array in PHP is an object in JSON).
		if typ == "" || (typ == "array" && n.children["*"] == nil) {
			typ = "object"
		}
		props := Schema{}
		var required []string
		for _, key := range named {
			c := n.children[key]
			props[key] = c.schema(res)
			if parse(c.rules).required {
				required = append(required, key)
			}
			if parse(c.rules).confirmed {
				props[key+"_confirmation"] = props[key]
				if parse(c.rules).required {
					required = append(required, key+"_confirmation")
				}
			}
		}
		s["properties"] = props
		if len(required) > 0 {
			s["required"] = required
		}
	}

	if typ != "" {
		if parsed.nullable {
			s["type"] = []string{typ, "null"}
		} else {
			s["type"] = typ
		}
	}
	// nullable without a known type needs no keyword: any value, null included.

	for k, v := range parsed.keywords(typ) {
		s[k] = v
	}
	if parsed.file {
		res.Multipart = true
	}
	if len(n.rules) > 0 {
		s["x-laravel-rules"] = n.rules
	}
	for _, r := range parsed.unmapped {
		res.Unmapped = append(res.Unmapped, n.field+": "+r)
	}
	return s
}

func (n *node) namedChildren() []string {
	var out []string
	for _, k := range n.order {
		if k != "*" {
			out = append(out, k)
		}
	}
	return out
}

// parsed is what one field's rule list says.
type parsed struct {
	typ       string // JSON Schema type, "" when unknown
	format    string
	file      bool
	required  bool
	nullable  bool
	confirmed bool
	min, max  *float64
	enum      []string
	notIn     []string
	pattern   string
	unmapped  []string
}

var (
	// Rules that only affect presence or are already covered elsewhere.
	presence = map[string]bool{"sometimes": true, "bail": true, "present": true, "filled": true}
	types    = map[string]string{
		"string": "string", "integer": "integer", "int": "integer", "numeric": "number",
		"decimal": "number", "boolean": "boolean", "bool": "boolean", "array": "array", "list": "array",
	}
	formats = map[string]string{
		"email": "email", "url": "uri", "active_url": "uri", "uuid": "uuid",
		"ipv4": "ipv4", "ipv6": "ipv6",
	}
	patterns = map[string]string{
		"ulid":       "^[0-9A-HJKMNP-TV-Za-hjkmnp-tv-z]{26}$",
		"alpha":      `^[\p{L}\p{M}]+$`,
		"alpha_num":  `^[\p{L}\p{M}\p{N}]+$`,
		"alpha_dash": `^[\p{L}\p{M}\p{N}_-]+$`,
		"lowercase":  `^[^\p{Lu}]*$`,
		"uppercase":  `^[^\p{Ll}]*$`,
	}
)

func parse(rules []string) parsed {
	var p parsed
	for _, rule := range rules {
		name, arg, _ := strings.Cut(rule, ":")

		switch {
		case name == "required":
			p.required = true
		case name == "nullable":
			p.nullable = true
		case name == "confirmed":
			p.confirmed = true
		case presence[name]:
		case types[name] != "":
			p.typ = types[name]
		case formats[name] != "":
			p.format = formats[name]
			if p.typ == "" {
				p.typ = "string"
			}
		case patterns[name] != "":
			p.pattern = patterns[name]
		case name == "date":
			// Any strtotime() string ("tomorrow", "2024-01-01"), so no format.
			p.typ = "string"
		case name == "date_format" && arg == "Y-m-d":
			p.typ, p.format = "string", "date"
		case name == "date_format" && arg == `Y-m-d\TH:i:sP`:
			p.typ, p.format = "string", "date-time"
		case name == "file" || name == "image" || name == "mimes" || name == "mimetypes":
			p.file = true
		case name == "min" || name == "max" || name == "size":
			if v, err := strconv.ParseFloat(arg, 64); err == nil {
				if name != "max" {
					p.min = &v
				}
				if name != "min" {
					p.max = &v
				}
			}
		case name == "between":
			lo, hi, _ := strings.Cut(arg, ",")
			if a, err := strconv.ParseFloat(lo, 64); err == nil {
				p.min = &a
			}
			if b, err := strconv.ParseFloat(hi, 64); err == nil {
				p.max = &b
			}
		case name == "in":
			p.enum = csvValues(arg)
		case name == "not_in":
			p.notIn = csvValues(arg)
		case name == "regex":
			if pat, ok := pcreToRE2(arg); ok {
				p.pattern = pat
			} else {
				p.unmapped = append(p.unmapped, rule)
			}
		case name == "accepted" || name == "declined":
			// Laravel accepts yes/on/1/true (or no/off/0/false): several types, no single schema.
			p.unmapped = append(p.unmapped, rule)
		default:
			// exists:, unique:, required_if:, rule objects (class names), Closure…
			p.unmapped = append(p.unmapped, rule)
		}
	}

	if p.file {
		p.typ, p.format = "string", "binary"
	}
	return p
}

// keywords maps size, enum and pattern rules for the field's type, the way
// Laravel's getSize() does: numbers compare values, arrays count items,
// files are kilobytes, anything else is string length.
func (p parsed) keywords(typ string) Schema {
	s := Schema{}
	if p.format != "" {
		s["format"] = p.format
	}
	if p.pattern != "" {
		s["pattern"] = p.pattern
	}

	switch {
	case p.file:
		// Size is in kilobytes of the upload; JSON Schema can't express that.
	case typ == "integer" || typ == "number":
		setNum(s, "minimum", p.min)
		setNum(s, "maximum", p.max)
	case typ == "array":
		setNum(s, "minItems", p.min)
		setNum(s, "maxItems", p.max)
	case typ == "object", typ == "boolean":
	default:
		setNum(s, "minLength", p.min)
		setNum(s, "maxLength", p.max)
	}

	if len(p.enum) > 0 {
		s["enum"] = typedValues(p.enum, typ)
	}
	if len(p.notIn) > 0 {
		s["not"] = Schema{"enum": typedValues(p.notIn, typ)}
	}
	return s
}

func setNum(s Schema, key string, v *float64) {
	if v == nil {
		return
	}
	if *v == float64(int64(*v)) {
		s[key] = int64(*v)
	} else {
		s[key] = *v
	}
}

// csvValues parses Laravel's in:/not_in: arguments: plain (a,b) or quoted
// the way Rule::in() writes them ("a","b, c").
func csvValues(arg string) []string {
	r := csv.NewReader(strings.NewReader(arg))
	r.LazyQuotes = true
	vals, err := r.Read()
	if err != nil {
		return strings.Split(arg, ",")
	}
	return vals
}

func typedValues(vals []string, typ string) []any {
	out := make([]any, 0, len(vals))
	for _, v := range vals {
		switch typ {
		case "integer":
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				out = append(out, i)
				continue
			}
		case "number":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				out = append(out, f)
				continue
			}
		}
		out = append(out, v)
	}
	return out
}

// pcreToRE2 turns "/^[a-z]+$/" into "^[a-z]+$". Patterns with flags (/i, /u…)
// or that Go's RE2 can't compile are left unmapped rather than mistranslated.
func pcreToRE2(arg string) (string, bool) {
	if len(arg) < 2 {
		return "", false
	}
	delim := arg[0]
	end := strings.LastIndexByte(arg, delim)
	if end <= 0 || end != len(arg)-1 {
		return "", false
	}
	pat := arg[1:end]
	if _, err := regexp.Compile(pat); err != nil {
		return "", false
	}
	return pat, true
}
