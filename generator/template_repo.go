package generator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"text/template/parse"
	"unicode"

	"github.com/Masterminds/sprig/v3"
	"github.com/kr/pretty"

	"github.com/go-openapi/inflect"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/swag"
)

var (
	assets             map[string][]byte
	protectedTemplates map[string]bool

	// FuncMapFunc yields a map with all functions for templates
	FuncMapFunc func(*LanguageOpts) template.FuncMap

	templates *Repository

	docFormat map[string]string

	errInternal = errors.New("internal error detected in templates")
)

func initTemplateRepo() {
	FuncMapFunc = DefaultFuncMap

	// this makes the ToGoName func behave with the special
	// prefixing rule above
	swag.GoNamePrefixFunc = prefixForName

	assets = defaultAssets()
	protectedTemplates = defaultProtectedTemplates()
	templates = NewRepository(FuncMapFunc(DefaultLanguageFunc()))

	docFormat = map[string]string{
		"binary": "binary (byte stream)",
		"byte":   "byte (base64 string)",
	}
}

// DefaultFuncMap yields a map with default functions for use in the templates.
// These are available in every template
func DefaultFuncMap(lang *LanguageOpts) template.FuncMap {
	f := sprig.TxtFuncMap()
	extra := template.FuncMap{
		"pascalize": pascalize,
		"camelize":  swag.ToJSONName,
		"varname":   lang.MangleVarName,
		"humanize":  swag.ToHumanNameLower,
		"snakize":   lang.MangleFileName,
		"toPackagePath": func(name string) string {
			return filepath.FromSlash(lang.ManglePackagePath(name, ""))
		},
		"toPackage": func(name string) string {
			return lang.ManglePackagePath(name, "")
		},
		"toPackageName": func(name string) string {
			return lang.ManglePackageName(name, "")
		},
		"dasherize":          swag.ToCommandName,
		"pluralizeFirstWord": pluralizeFirstWord,
		"json":               asJSON,
		"prettyjson":         asPrettyJSON,
		"hasInsecure": func(arg []string) bool {
			return swag.ContainsStringsCI(arg, "http") || swag.ContainsStringsCI(arg, "ws")
		},
		"hasSecure": func(arg []string) bool {
			return swag.ContainsStringsCI(arg, "https") || swag.ContainsStringsCI(arg, "wss")
		},
		"dropPackage":      dropPackage,
		"containsPkgStr":   containsPkgStr,
		"contains":         swag.ContainsStrings,
		"padSurround":      padSurround,
		"joinFilePath":     filepath.Join,
		"joinPath":         path.Join,
		"comment":          padComment,
		"blockcomment":     blockComment,
		"inspect":          pretty.Sprint,
		"cleanPath":        path.Clean,
		"mediaTypeName":    mediaMime,
		"mediaGoName":      mediaGoName,
		"arrayInitializer": lang.arrayInitializer,
		"hasPrefix":        strings.HasPrefix,
		"stringContains":   strings.Contains,
		"imports":          lang.imports,
		"dict":             dict,
		"isInteger":        isInteger,
		"escapeBackticks": func(arg string) string {
			return strings.ReplaceAll(arg, "`", "`+\"`\"+`")
		},
		"paramDocType": func(param GenParameter) string {
			return resolvedDocType(param.SwaggerType, param.SwaggerFormat, param.Child)
		},
		"headerDocType": func(header GenHeader) string {
			return resolvedDocType(header.SwaggerType, header.SwaggerFormat, header.Child)
		},
		"schemaDocType": func(in any) string {
			switch schema := in.(type) {
			case GenSchema:
				return resolvedDocSchemaType(schema.SwaggerType, schema.SwaggerFormat, schema.Items)
			case *GenSchema:
				if schema == nil {
					return ""
				}
				return resolvedDocSchemaType(schema.SwaggerType, schema.SwaggerFormat, schema.Items)
			case GenDefinition:
				return resolvedDocSchemaType(schema.SwaggerType, schema.SwaggerFormat, schema.Items)
			case *GenDefinition:
				if schema == nil {
					return ""
				}
				return resolvedDocSchemaType(schema.SwaggerType, schema.SwaggerFormat, schema.Items)
			default:
				panic("dev error: schemaDocType should be called with GenSchema or GenDefinition")
			}
		},
		"schemaDocMapType": func(schema GenSchema) string {
			return resolvedDocElemType("object", schema.SwaggerFormat, &schema.resolvedType)
		},
		"docCollectionFormat": resolvedDocCollectionFormat,
		"trimSpace":           strings.TrimSpace,
		"mdBlock":             markdownBlock, // markdown block
		"httpStatus":          httpStatus,
		"cleanupEnumVariant":  cleanupEnumVariant,
		"gt0":                 gt0,
		"path":                errorPath,
		"cmdName": func(in any) (string, error) {
			// builds the name of a CLI command for a single operation
			op, isOperation := in.(GenOperation)
			if !isOperation {
				ptr, ok := in.(*GenOperation)
				if !ok {
					return "", fmt.Errorf("cmdName should be called on a GenOperation, but got: %T", in)
				}
				op = *ptr
			}
			name := "Operation" + pascalize(op.Package) + pascalize(op.Name) + "Cmd"

			return name, nil // TODO
		},
		"cmdGroupName": func(in any) (string, error) {
			// builds the name of a group of CLI commands
			opGroup, ok := in.(GenOperationGroup)
			if !ok {
				return "", fmt.Errorf("cmdGroupName should be called on a GenOperationGroup, but got: %T", in)
			}
			name := "GroupOfOperations" + pascalize(opGroup.Name) + "Cmd"

			return name, nil // TODO
		},
		"flagNameVar": func(in string) string {
			// builds a flag name variable in CLI commands
			return fmt.Sprintf("flag%sName", pascalize(in))
		},
		"flagValueVar": func(in string) string {
			// builds a flag value variable in CLI commands
			return fmt.Sprintf("flag%sValue", pascalize(in))
		},
		"flagDefaultVar": func(in string) string {
			// builds a flag default value variable in CLI commands
			return fmt.Sprintf("flag%sDefault", pascalize(in))
		},
		"flagModelVar": func(in string) string {
			// builds a flag model variable in CLI commands
			return fmt.Sprintf("flag%sModel", pascalize(in))
		},
		"flagDescriptionVar": func(in string) string {
			// builds a flag description variable in CLI commands
			return fmt.Sprintf("flag%sDescription", pascalize(in))
		},
		"printGoLiteral": func(in any) string {
			// printGoLiteral replaces printf "%#v" and replaces "interface {}" by "any"
			return interfaceReplacer.Replace(fmt.Sprintf("%#v", in))
		},
		// assert is used to inject into templates and check for inconsistent/invalid data.
		// This is for now being used during test & debug of templates.
		"assert": func(msg string, assertion bool) (string, error) {
			if !assertion {
				return "", fmt.Errorf("%v: %w", msg, errInternal)
			}

			return "", nil
		},
	}

	for k, v := range extra {
		f[k] = v
	}

	return f
}

func defaultAssets() map[string][]byte {
	return map[string][]byte{
		// schema validation templates
		"validation/primitive.gotmpl":    MustAsset("templates/validation/primitive.gotmpl"),
		"validation/customformat.gotmpl": MustAsset("templates/validation/customformat.gotmpl"),
		"validation/structfield.gotmpl":  MustAsset("templates/validation/structfield.gotmpl"),
		"structfield.gotmpl":             MustAsset("templates/structfield.gotmpl"),
		"schemavalidator.gotmpl":         MustAsset("templates/schemavalidator.gotmpl"),
		"schemapolymorphic.gotmpl":       MustAsset("templates/schemapolymorphic.gotmpl"),
		"schemaembedded.gotmpl":          MustAsset("templates/schemaembedded.gotmpl"),
		"validation/minimum.gotmpl":      MustAsset("templates/validation/minimum.gotmpl"),
		"validation/maximum.gotmpl":      MustAsset("templates/validation/maximum.gotmpl"),
		"validation/multipleOf.gotmpl":   MustAsset("templates/validation/multipleOf.gotmpl"),

		// schema serialization templates
		"additionalpropertiesserializer.gotmpl": MustAsset("templates/serializers/additionalpropertiesserializer.gotmpl"),
		"aliasedserializer.gotmpl":              MustAsset("templates/serializers/aliasedserializer.gotmpl"),
		"allofserializer.gotmpl":                MustAsset("templates/serializers/allofserializer.gotmpl"),
		"basetypeserializer.gotmpl":             MustAsset("templates/serializers/basetypeserializer.gotmpl"),
		"marshalbinaryserializer.gotmpl":        MustAsset("templates/serializers/marshalbinaryserializer.gotmpl"),
		"schemaserializer.gotmpl":               MustAsset("templates/serializers/schemaserializer.gotmpl"),
		"subtypeserializer.gotmpl":              MustAsset("templates/serializers/subtypeserializer.gotmpl"),
		"tupleserializer.gotmpl":                MustAsset("templates/serializers/tupleserializer.gotmpl"),

		// schema generation template
		"docstring.gotmpl":  MustAsset("templates/docstring.gotmpl"),
		"schematype.gotmpl": MustAsset("templates/schematype.gotmpl"),
		"schemabody.gotmpl": MustAsset("templates/schemabody.gotmpl"),
		"schema.gotmpl":     MustAsset("templates/schema.gotmpl"),
		"model.gotmpl":      MustAsset("templates/model.gotmpl"),
		"header.gotmpl":     MustAsset("templates/header.gotmpl"),

		// simple schema generation helpers templates
		"simpleschema/defaultsvar.gotmpl":  MustAsset("templates/simpleschema/defaultsvar.gotmpl"),
		"simpleschema/defaultsinit.gotmpl": MustAsset("templates/simpleschema/defaultsinit.gotmpl"),

		"swagger_json_embed.gotmpl": MustAsset("templates/swagger_json_embed.gotmpl"),

		// server templates
		"server/parameter.gotmpl":        MustAsset("templates/server/parameter.gotmpl"),
		"server/urlbuilder.gotmpl":       MustAsset("templates/server/urlbuilder.gotmpl"),
		"server/responses.gotmpl":        MustAsset("templates/server/responses.gotmpl"),
		"server/operation.gotmpl":        MustAsset("templates/server/operation.gotmpl"),
		"server/builder.gotmpl":          MustAsset("templates/server/builder.gotmpl"),
		"server/server.gotmpl":           MustAsset("templates/server/server.gotmpl"),
		"server/configureapi.gotmpl":     MustAsset("templates/server/configureapi.gotmpl"),
		"server/autoconfigureapi.gotmpl": MustAsset("templates/server/autoconfigureapi.gotmpl"),
		"server/main.gotmpl":             MustAsset("templates/server/main.gotmpl"),
		"server/doc.gotmpl":              MustAsset("templates/server/doc.gotmpl"),

		// client templates
		"client/parameter.gotmpl": MustAsset("templates/client/parameter.gotmpl"),
		"client/response.gotmpl":  MustAsset("templates/client/response.gotmpl"),
		"client/client.gotmpl":    MustAsset("templates/client/client.gotmpl"),
		"client/facade.gotmpl":    MustAsset("templates/client/facade.gotmpl"),

		"markdown/docs.gotmpl": MustAsset("templates/markdown/docs.gotmpl"),

		// cli templates
		"cli/cli.gotmpl":          MustAsset("templates/cli/cli.gotmpl"),
		"cli/main.gotmpl":         MustAsset("templates/cli/main.gotmpl"),
		"cli/modelcli.gotmpl":     MustAsset("templates/cli/modelcli.gotmpl"),
		"cli/operation.gotmpl":    MustAsset("templates/cli/operation.gotmpl"),
		"cli/registerflag.gotmpl": MustAsset("templates/cli/registerflag.gotmpl"),
		"cli/retrieveflag.gotmpl": MustAsset("templates/cli/retrieveflag.gotmpl"),
		"cli/schema.gotmpl":       MustAsset("templates/cli/schema.gotmpl"),
		"cli/completion.gotmpl":   MustAsset("templates/cli/completion.gotmpl"),
	}
}

func defaultProtectedTemplates() map[string]bool {
	return map[string]bool{
		"dereffedSchemaType":          true,
		"docstring":                   true,
		"header":                      true,
		"mapvalidator":                true,
		"model":                       true,
		"modelvalidator":              true,
		"objectvalidator":             true,
		"primitivefieldvalidator":     true,
		"privstructfield":             true,
		"privtuplefield":              true,
		"propertyValidationDocString": true,
		"propertyvalidator":           true,
		"schema":                      true,
		"schemaBody":                  true,
		"schemaType":                  true,
		"schemabody":                  true,
		"schematype":                  true,
		"schemavalidator":             true,
		"serverDoc":                   true,
		"slicevalidator":              true,
		"structfield":                 true,
		"structfieldIface":            true,
		"subTypeBody":                 true,
		"swaggerJsonEmbed":            true,
		"tuplefield":                  true,
		"tuplefieldIface":             true,
		"typeSchemaType":              true,
		"simpleschemaDefaultsvar":     true,
		"simpleschemaDefaultsinit":    true,

		// validation helpers
		"validationCustomformat": true,
		"validationPrimitive":    true,
		"validationStructfield":  true,
		"withBaseTypeBody":       true,
		"withoutBaseTypeBody":    true,
		"validationMinimum":      true,
		"validationMaximum":      true,
		"validationMultipleOf":   true,

		// all serializers
		"additionalPropertiesSerializer": true,
		"tupleSerializer":                true,
		"schemaSerializer":               true,
		"hasDiscriminatedSerializer":     true,
		"discriminatedSerializer":        true,
	}
}

// AddFile adds a file to the default repository. It will create a new template based on the filename.
// It trims the .gotmpl from the end and converts the name using swag.ToJSONName. This will strip
// directory separators and Camelcase the next letter.
// e.g validation/primitive.gotmpl will become validationPrimitive
//
// If the file contains a definition for a template that is protected the whole file will not be added
func AddFile(name, data string) error {
	return templates.addFile(name, data, false)
}

// NewRepository creates a new template repository with the provided functions defined
func NewRepository(funcs template.FuncMap) *Repository {
	repo := Repository{
		files:     make(map[string]string),
		templates: make(map[string]*template.Template),
		funcs:     funcs,
	}

	if repo.funcs == nil {
		repo.funcs = make(template.FuncMap)
	}

	return &repo
}

// Repository is the repository for the generator templates
type Repository struct {
	files         map[string]string
	templates     map[string]*template.Template
	funcs         template.FuncMap
	allowOverride bool
	mux           sync.Mutex
}

// ShallowClone a repository.
//
// Clones the maps of files and templates, so as to be able to use
// the cloned repo concurrently.
func (t *Repository) ShallowClone() *Repository {
	clone := &Repository{
		files:         make(map[string]string, len(t.files)),
		templates:     make(map[string]*template.Template, len(t.templates)),
		funcs:         t.funcs,
		allowOverride: t.allowOverride,
	}

	t.mux.Lock()
	defer t.mux.Unlock()

	for k, file := range t.files {
		clone.files[k] = file
	}
	for k, tpl := range t.templates {
		clone.templates[k] = tpl
	}
	return clone
}

// LoadDefaults will load the embedded templates
func (t *Repository) LoadDefaults() {
	for name, asset := range assets {
		if err := t.addFile(name, string(asset), true); err != nil {
			log.Fatal(err)
		}
	}
}

// LoadDir will walk the specified path and add each .gotmpl file it finds to the repository
func (t *Repository) LoadDir(templatePath string) error {
	err := filepath.Walk(templatePath, func(path string, _ os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".gotmpl") {
			if assetName, e := filepath.Rel(templatePath, path); e == nil {
				if data, e := os.ReadFile(path); e == nil {
					if ee := t.AddFile(assetName, string(data)); ee != nil {
						return fmt.Errorf("could not add template: %w", ee)
					}
				}
				// Non-readable files are skipped
			}
		}

		if err != nil {
			return err
		}

		// Non-template files are skipped
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not complete template processing in directory \"%s\": %w", templatePath, err)
	}
	return nil
}

// LoadContrib loads template from contrib directory
func (t *Repository) LoadContrib(name string) error {
	log.Printf("loading contrib %s", name)
	const pathPrefix = "templates/contrib/"
	basePath := pathPrefix + name
	filesAdded := 0
	for _, aname := range AssetNames() {
		if !strings.HasSuffix(aname, ".gotmpl") {
			continue
		}
		if strings.HasPrefix(aname, basePath) {
			target := aname[len(basePath)+1:]
			err := t.addFile(target, string(MustAsset(aname)), true)
			if err != nil {
				return err
			}
			log.Printf("added contributed template %s from %s", target, aname)
			filesAdded++
		}
	}
	if filesAdded == 0 {
		return fmt.Errorf("no files added from template: %s", name)
	}
	return nil
}

func (t *Repository) addFile(name, data string, allowOverride bool) error {
	fileName := name
	name = swag.ToJSONName(strings.TrimSuffix(name, ".gotmpl"))

	templ, err := template.New(name).Funcs(t.funcs).Parse(data)
	if err != nil {
		return fmt.Errorf("failed to load template %s: %w", name, err)
	}

	// check if any protected templates are defined
	if !allowOverride && !t.allowOverride {
		for _, template := range templ.Templates() {
			if protectedTemplates[template.Name()] {
				return fmt.Errorf("cannot overwrite protected template %s", template.Name())
			}
		}
	}

	// Add each defined template into the cache
	for _, template := range templ.Templates() {

		t.files[template.Name()] = fileName
		t.templates[template.Name()] = template.Lookup(template.Name())
	}

	return nil
}

// MustGet a template by name, panics when fails
func (t *Repository) MustGet(name string) *template.Template {
	tpl, err := t.Get(name)
	if err != nil {
		panic(err)
	}
	return tpl
}

// AddFile adds a file to the repository. It will create a new template based on the filename.
// It trims the .gotmpl from the end and converts the name using swag.ToJSONName. This will strip
// directory separators and Camelcase the next letter.
// e.g validation/primitive.gotmpl will become validationPrimitive
//
// If the file contains a definition for a template that is protected the whole file will not be added
func (t *Repository) AddFile(name, data string) error {
	return t.addFile(name, data, false)
}

// SetAllowOverride allows setting allowOverride after the Repository was initialized
func (t *Repository) SetAllowOverride(value bool) {
	t.allowOverride = value
}

func findDependencies(n parse.Node) []string {
	var deps []string
	depMap := make(map[string]bool)

	if n == nil {
		return deps
	}

	switch node := n.(type) {
	case *parse.ListNode:
		if node != nil && node.Nodes != nil {
			for _, nn := range node.Nodes {
				for _, dep := range findDependencies(nn) {
					depMap[dep] = true
				}
			}
		}
	case *parse.IfNode:
		for _, dep := range findDependencies(node.List) {
			depMap[dep] = true
		}
		for _, dep := range findDependencies(node.ElseList) {
			depMap[dep] = true
		}

	case *parse.RangeNode:
		for _, dep := range findDependencies(node.List) {
			depMap[dep] = true
		}
		for _, dep := range findDependencies(node.ElseList) {
			depMap[dep] = true
		}

	case *parse.WithNode:
		for _, dep := range findDependencies(node.List) {
			depMap[dep] = true
		}
		for _, dep := range findDependencies(node.ElseList) {
			depMap[dep] = true
		}

	case *parse.TemplateNode:
		depMap[node.Name] = true
	}

	for dep := range depMap {
		deps = append(deps, dep)
	}

	return deps
}

func (t *Repository) flattenDependencies(templ *template.Template, dependencies map[string]bool) map[string]bool {
	if dependencies == nil {
		dependencies = make(map[string]bool)
	}

	deps := findDependencies(templ.Root)

	for _, d := range deps {
		if _, found := dependencies[d]; !found {

			dependencies[d] = true

			if tt := t.templates[d]; tt != nil {
				dependencies = t.flattenDependencies(tt, dependencies)
			}
		}

		dependencies[d] = true

	}

	return dependencies
}

func (t *Repository) addDependencies(templ *template.Template) (*template.Template, error) {
	name := templ.Name()

	deps := t.flattenDependencies(templ, nil)

	for dep := range deps {

		if dep == "" {
			continue
		}

		tt := templ.Lookup(dep)

		// Check if we have it
		if tt == nil {
			tt = t.templates[dep]

			// Still don't have it, return an error
			if tt == nil {
				return templ, fmt.Errorf("could not find template %s", dep)
			}
			var err error

			// Add it to the parse tree
			templ, err = templ.AddParseTree(dep, tt.Tree)
			if err != nil {
				return templ, fmt.Errorf("dependency error: %w", err)
			}

		}
	}
	return templ.Lookup(name), nil
}

// Get will return the named template from the repository, ensuring that all dependent templates are loaded.
// It will return an error if a dependent template is not defined in the repository.
func (t *Repository) Get(name string) (*template.Template, error) {
	templ, found := t.templates[name]

	if !found {
		return templ, fmt.Errorf("template doesn't exist %s", name)
	}

	return t.addDependencies(templ)
}

// DumpTemplates prints out a dump of all the defined templates, where they are defined and what their dependencies are.
func (t *Repository) DumpTemplates() {
	buf := bytes.NewBuffer(nil)
	fmt.Fprintln(buf, "\n# Templates")
	for name, templ := range t.templates {
		fmt.Fprintf(buf, "## %s\n", name)
		fmt.Fprintf(buf, "Defined in `%s`\n", t.files[name])

		if deps := findDependencies(templ.Root); len(deps) > 0 {
			fmt.Fprintf(buf, "####requires \n - %v\n\n\n", strings.Join(deps, "\n - "))
		}
		fmt.Fprintln(buf, "\n---")
	}
	log.Println(buf.String())
}

// FuncMap functions

func asJSON(data any) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func asPrettyJSON(data any) (string, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func pluralizeFirstWord(arg string) string {
	sentence := strings.Split(arg, " ")
	if len(sentence) == 1 {
		return inflect.Pluralize(arg)
	}

	return inflect.Pluralize(sentence[0]) + " " + strings.Join(sentence[1:], " ")
}

func dropPackage(str string) string {
	parts := strings.Split(str, ".")
	return parts[len(parts)-1]
}

// return true if the GoType str contains pkg. For example "model.MyType" -> true, "MyType" -> false
func containsPkgStr(str string) bool {
	dropped := dropPackage(str)
	return dropped != str
}

func padSurround(entry, padWith string, i, ln int) string {
	var res []string
	if i > 0 {
		for j := 0; j < i; j++ {
			res = append(res, padWith)
		}
	}
	res = append(res, entry)
	tot := ln - i - 1
	for j := 0; j < tot; j++ {
		res = append(res, padWith)
	}
	return strings.Join(res, ",")
}

func padComment(str string, pads ...string) string {
	// pads specifes padding to indent multi line comments.Defaults to one space
	pad := " "
	lines := strings.Split(str, "\n")
	if len(pads) > 0 {
		pad = strings.Join(pads, "")
	}
	return (strings.Join(lines, "\n//"+pad))
}

func blockComment(str string) string {
	return strings.ReplaceAll(str, "*/", "[*]/")
}

func pascalize(arg string) string {
	runes := []rune(arg)
	switch len(runes) {
	case 0:
		return "Empty"
	case 1: // handle special case when we have a single rune that is not handled by swag.ToGoName
		switch runes[0] {
		case '+', '-', '#', '_', '*', '/', '=': // those cases are handled differently than swag utility
			return prefixForName(arg)
		}
	}
	return swag.ToGoName(swag.ToGoName(arg)) // want to remove spaces
}

func prefixForName(arg string) string {
	first := []rune(arg)[0]
	if len(arg) == 0 || unicode.IsLetter(first) {
		return ""
	}
	switch first {
	case '+':
		return "Plus"
	case '-':
		return "Minus"
	case '#':
		return "HashTag"
	case '*':
		return "Asterisk"
	case '/':
		return "ForwardSlash"
	case '=':
		return "EqualSign"
		// other cases ($,@ etc..) handled by swag.ToGoName
	}
	return "Nr"
}

func replaceSpecialChar(in rune) string {
	switch in {
	case '.':
		return "-Dot-"
	case '+':
		return "-Plus-"
	case '-':
		return "-Dash-"
	case '#':
		return "-Hashtag-"
	}
	return string(in)
}

func cleanupEnumVariant(in string) string {
	replaced := ""
	for _, char := range in {
		replaced += replaceSpecialChar(char)
	}
	return replaced
}

func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("expected even number of arguments, got %d", len(values))
	}
	dict := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %+v", values[i])
		}
		dict[key] = values[i+1]
	}
	return dict, nil
}

func isInteger(arg any) bool {
	// is integer determines if a value may be represented by an integer
	switch val := arg.(type) {
	case int8, int16, int32, int, int64, uint8, uint16, uint32, uint, uint64:
		return true
	case *int8, *int16, *int32, *int, *int64, *uint8, *uint16, *uint32, *uint, *uint64:
		v := reflect.ValueOf(arg)
		return !v.IsNil()
	case float64:
		return math.Round(val) == val
	case *float64:
		return val != nil && math.Round(*val) == *val
	case float32:
		return math.Round(float64(val)) == float64(val)
	case *float32:
		return val != nil && math.Round(float64(*val)) == float64(*val)
	case string:
		_, err := strconv.ParseInt(val, 10, 64)
		return err == nil
	case *string:
		if val == nil {
			return false
		}
		_, err := strconv.ParseInt(*val, 10, 64)
		return err == nil
	default:
		return false
	}
}

func resolvedDocCollectionFormat(cf string, child *GenItems) string {
	if child == nil {
		return cf
	}
	ccf := cf
	if ccf == "" {
		ccf = "csv"
	}
	rcf := resolvedDocCollectionFormat(child.CollectionFormat, child.Child)
	if rcf == "" {
		return ccf
	}
	return ccf + "|" + rcf
}

func resolvedDocType(tn, ft string, child *GenItems) string {
	if tn == "array" {
		if child == nil {
			return "[]any"
		}
		return "[]" + resolvedDocType(child.SwaggerType, child.SwaggerFormat, child.Child)
	}

	if ft != "" {
		if doc, ok := docFormat[ft]; ok {
			return doc
		}
		return fmt.Sprintf("%s (formatted %s)", ft, tn)
	}

	return tn
}

func resolvedDocSchemaType(tn, ft string, child *GenSchema) string {
	if tn == "array" {
		if child == nil {
			return "[]any"
		}
		return "[]" + resolvedDocSchemaType(child.SwaggerType, child.SwaggerFormat, child.Items)
	}

	if tn == "object" {
		if child == nil || child.ElemType == nil {
			return "map of any"
		}
		if child.IsMap {
			return "map of " + resolvedDocElemType(child.SwaggerType, child.SwaggerFormat, &child.resolvedType)
		}

		return child.GoType
	}

	if ft != "" {
		if doc, ok := docFormat[ft]; ok {
			return doc
		}
		return fmt.Sprintf("%s (formatted %s)", ft, tn)
	}

	return tn
}

func resolvedDocElemType(tn, ft string, schema *resolvedType) string {
	if schema == nil {
		return ""
	}
	if schema.IsMap {
		return "map of " + resolvedDocElemType(schema.ElemType.SwaggerType, schema.ElemType.SwaggerFormat, schema.ElemType)
	}

	if schema.IsArray {
		return "[]" + resolvedDocElemType(schema.ElemType.SwaggerType, schema.ElemType.SwaggerFormat, schema.ElemType)
	}

	if ft != "" {
		if doc, ok := docFormat[ft]; ok {
			return doc
		}
		return fmt.Sprintf("%s (formatted %s)", ft, tn)
	}

	return tn
}

func httpStatus(code int) string {
	if name, ok := runtime.Statuses[code]; ok {
		return name
	}
	// non-standard codes deserve some name
	return fmt.Sprintf("Status %d", code)
}

func gt0(in *int64) bool {
	// gt0 returns true if the *int64 points to a value > 0
	// NOTE: plain {{ gt .MinProperties 0 }} just refuses to work normally
	// with a pointer
	return in != nil && *in > 0
}

func errorPath(in any) (string, error) {
	// For schemas:
	// errorPath returns an empty string litteral when the schema path is empty.
	// It provides a shorthand for template statements such as:
	// {{ if .Path }}{{ .Path }}{{ else }}" "{{ end }},
	// which becomes {{ path . }}
	//
	// When called for a GenParameter, GenResponse or GenOperation object, it just
	// returns Path.
	//
	// Extra behavior for schemas, when the generation option RootedErroPath is enabled:
	// In the case of arrays with an empty path, it adds the type name as the path "root",
	// so consumers of reported errors get an idea of the originator.

	var pth string
	rooted := func(schema GenSchema) string {
		if schema.WantsRootedErrorPath && schema.Path == "" && (schema.IsArray || schema.IsMap) {
			return `"[` + schema.Name + `]"`
		}

		return schema.Path
	}

	switch schema := in.(type) {
	case GenSchema:
		pth = rooted(schema)
	case *GenSchema:
		if schema == nil {
			break
		}
		pth = rooted(*schema)
	case GenDefinition:
		pth = rooted(schema.GenSchema)
	case *GenDefinition:
		if schema == nil {
			break
		}
		pth = rooted(schema.GenSchema)
	case GenParameter:
		pth = schema.Path

	// unchanged Path if called with other types
	case *GenParameter:
		if schema == nil {
			break
		}
		pth = schema.Path
	case GenResponse:
		pth = schema.Path
	case *GenResponse:
		if schema == nil {
			break
		}
		pth = schema.Path
	case GenOperation:
		pth = schema.Path
	case *GenOperation:
		if schema == nil {
			break
		}
		pth = schema.Path
	case GenItems:
		pth = schema.Path
	case *GenItems:
		if schema == nil {
			break
		}
		pth = schema.Path
	case GenHeader:
		pth = schema.Path
	case *GenHeader:
		if schema == nil {
			break
		}
		pth = schema.Path
	default:
		return "", fmt.Errorf("errorPath should be called with GenSchema or GenDefinition, but got %T", schema)
	}

	if pth == "" {
		return `""`, nil
	}

	return pth, nil
}

const mdNewLine = "</br>"

var (
	mdNewLineReplacer = strings.NewReplacer("\r\n", mdNewLine, "\n", mdNewLine, "\r", mdNewLine)
	interfaceReplacer = strings.NewReplacer("interface {}", "any")
)

func markdownBlock(in string) string {
	in = strings.TrimSpace(in)

	return mdNewLineReplacer.Replace(in)
}
