package v1

import (
	_ "embed"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed openapi.base.yaml
var baseSpec []byte

// Access says who may call an operation. Contracts set it per operation
// with x-access, users is the default.
type Access string

const (
	AccessUser   Access = "user"
	AccessAdmin  Access = "admin"
	AccessPublic Access = "public"
)

var methods = []string{"get", "put", "post", "delete", "patch", "head", "options"}

var pathParam = regexp.MustCompile(`\{([^}]+)\}`)

// Operation is one method on one path, as written in a contract.
type Operation struct {
	Method string
	Path   string
	ID     string
	Tag    string
	Access Access

	node    *yaml.Node
	handler HandlerFunc
}

// Pattern is the operation as a net/http pattern, for example "GET /fs/list".
func (o *Operation) Pattern() string { return strings.ToUpper(o.Method) + " " + o.Path }

// Implemented reports whether a package bound a handler to the operation.
func (o *Operation) Implemented() bool { return o.handler != nil }

type fragment struct {
	name       string
	tag        *yaml.Node
	paths      *yaml.Node
	components *yaml.Node
}

// parseFragment reads one package contract. A fragment has a tag with name
// and description, paths, and optionally components.
func parseFragment(name string, data []byte) (*fragment, []*Operation, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", name, err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("%s: contract must be a mapping", name)
	}
	f := &fragment{name: name}
	root := doc.Content[0]
	f.tag = get(root, "tag")
	f.paths = get(root, "paths")
	f.components = get(root, "components")
	tagName := ""
	if f.tag != nil {
		if n := get(f.tag, "name"); n != nil {
			tagName = n.Value
		}
	}
	if tagName == "" {
		return nil, nil, fmt.Errorf("%s: tag.name is missing", name)
	}
	if f.paths == nil || f.paths.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("%s: paths are missing", name)
	}
	var ops []*Operation
	for i := 0; i < len(f.paths.Content); i += 2 {
		p, item := f.paths.Content[i].Value, f.paths.Content[i+1]
		if !strings.HasPrefix(p, "/") {
			return nil, nil, fmt.Errorf("%s: path %q must start with /", name, p)
		}
		for j := 0; j < len(item.Content); j += 2 {
			method, opNode := item.Content[j].Value, item.Content[j+1]
			if !slices.Contains(methods, method) {
				continue
			}
			op := &Operation{Method: method, Path: p, Tag: tagName, Access: AccessUser, node: opNode}
			if n := get(opNode, "operationId"); n != nil {
				op.ID = n.Value
			}
			if op.ID == "" {
				return nil, nil, fmt.Errorf("%s: %s %s has no operationId", name, method, p)
			}
			if n := get(opNode, "x-access"); n != nil {
				op.Access = Access(n.Value)
			}
			switch op.Access {
			case AccessUser, AccessAdmin, AccessPublic:
			default:
				return nil, nil, fmt.Errorf("%s: %s has unknown x-access %q", name, op.ID, op.Access)
			}
			normalize(op)
			ops = append(ops, op)
		}
	}
	return f, ops, nil
}

// normalize fills in what every operation needs, so contracts can stay
// short: the tag, the path parameters, a 501 answer while nothing is
// implemented, and the security for public operations.
func normalize(op *Operation) {
	n := op.node
	if get(n, "tags") == nil {
		set(n, "tags", &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle, Content: []*yaml.Node{str(op.Tag)}})
	}
	declared := map[string]bool{}
	params := get(n, "parameters")
	if params != nil {
		for _, p := range params.Content {
			if in := get(p, "in"); in != nil && in.Value == "path" {
				declared[get(p, "name").Value] = true
			}
		}
	}
	for _, m := range pathParam.FindAllStringSubmatch(op.Path, -1) {
		if declared[m[1]] {
			continue
		}
		if params == nil {
			params = &yaml.Node{Kind: yaml.SequenceNode}
			set(n, "parameters", params)
		}
		params.Content = append(params.Content, mapping(
			"name", str(m[1]), "in", str("path"), "required", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
			"schema", mapping("type", str("string")),
		))
	}
	if get(n, "responses") == nil {
		set(n, "responses", mapping("501", mapping("$ref", str("#/components/responses/NotImplemented"))))
	}
	if op.Access == AccessPublic && get(n, "security") == nil {
		set(n, "security", &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle})
	}
}

// merge builds the full OpenAPI document from the base and the fragments.
func merge(frags []*fragment) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(baseSpec, &doc); err != nil {
		return nil, fmt.Errorf("base contract: %w", err)
	}
	root := doc.Content[0]
	tags := get(root, "tags")
	if tags == nil {
		tags = &yaml.Node{Kind: yaml.SequenceNode}
		set(root, "tags", tags)
	}
	paths := get(root, "paths")
	if paths == nil {
		paths = &yaml.Node{Kind: yaml.MappingNode}
		set(root, "paths", paths)
	}
	// The base writes them as empty flow collections to fix their position.
	tags.Style, paths.Style = 0, 0
	components := get(root, "components")
	for _, f := range frags {
		tags.Content = append(tags.Content, f.tag)
		for i := 0; i < len(f.paths.Content); i += 2 {
			key := f.paths.Content[i].Value
			if get(paths, key) != nil {
				return nil, fmt.Errorf("%s: path %s is already declared", f.name, key)
			}
			paths.Content = append(paths.Content, f.paths.Content[i], f.paths.Content[i+1])
		}
		if f.components == nil {
			continue
		}
		for i := 0; i < len(f.components.Content); i += 2 {
			section, entries := f.components.Content[i].Value, f.components.Content[i+1]
			target := get(components, section)
			if target == nil {
				target = &yaml.Node{Kind: yaml.MappingNode}
				set(components, section, target)
			}
			for j := 0; j < len(entries.Content); j += 2 {
				name := entries.Content[j].Value
				if get(target, name) != nil {
					return nil, fmt.Errorf("%s: components.%s.%s is already declared", f.name, section, name)
				}
				target.Content = append(target.Content, entries.Content[j], entries.Content[j+1])
			}
		}
	}
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func get(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func set(m *yaml.Node, key string, v *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = v
			return
		}
	}
	m.Content = append(m.Content, str(key), v)
}

func str(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }

func mapping(kv ...any) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i < len(kv); i += 2 {
		n.Content = append(n.Content, str(kv[i].(string)), kv[i+1].(*yaml.Node))
	}
	return n
}
