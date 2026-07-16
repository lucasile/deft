package recipes

import (
	"embed"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

//go:embed builtin/*.json
var builtinRecipeFiles embed.FS

type EditableBy string

const (
	EditableByUser  EditableBy = "user"
	EditableByAdmin EditableBy = "admin"
)

type InputType string

const (
	InputString InputType = "string"
	InputNumber InputType = "number"
	InputSelect InputType = "select"
)

type Input struct {
	Key        string     `json:"key"`
	Label      string     `json:"label"`
	Type       InputType  `json:"type"`
	Default    any        `json:"default,omitempty"`
	Required   bool       `json:"required"`
	EditableBy EditableBy `json:"editable_by"`
	Min        *int       `json:"min,omitempty"`
	Max        *int       `json:"max,omitempty"`
	Options    []string   `json:"options,omitempty"`
}

type Recipe struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Version     string  `json:"version"`
	Source      string  `json:"source"`
	Enabled     bool    `json:"enabled"`
	Inputs      []Input `json:"inputs"`
}

type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only,omitempty"`
}

type Rendered struct {
	RecipeID      string         `json:"recipe_id"`
	RecipeVersion string         `json:"recipe_version"`
	Name          string         `json:"name"`
	Image         string         `json:"image"`
	Values        map[string]any `json:"values"`
	Ports         []PortMapping  `json:"ports,omitempty"`
	Env           []EnvVar       `json:"env,omitempty"`
	Volumes       []VolumeMount  `json:"volumes,omitempty"`
	RestartPolicy string         `json:"restart_policy,omitempty"`
}

type RenderedConfig struct {
	RecipeID      string         `json:"recipe_id"`
	RecipeVersion string         `json:"recipe_version"`
	Values        map[string]any `json:"values"`
	Rendered      Rendered       `json:"rendered"`
}

type definition struct {
	Recipe
	Container containerTemplate `json:"container"`
}

type containerTemplate struct {
	Name          string           `json:"name"`
	Image         string           `json:"image"`
	Ports         []portTemplate   `json:"ports,omitempty"`
	Env           []envTemplate    `json:"env,omitempty"`
	Volumes       []volumeTemplate `json:"volumes,omitempty"`
	RestartPolicy string           `json:"restart_policy,omitempty"`
}

type portTemplate struct {
	HostPort      any    `json:"host_port"`
	ContainerPort any    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

type envTemplate struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type volumeTemplate struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only,omitempty"`
}

var builtinDefinitions = mustLoadBuiltinDefinitions()

func List() []Recipe {
	result := make([]Recipe, 0, len(builtinDefinitions))
	for _, def := range builtinDefinitions {
		if def.Enabled {
			result = append(result, def.Recipe)
		}
	}
	return result
}

func Get(id string) (Recipe, bool) {
	def, ok := getDefinition(id)
	if !ok {
		return Recipe{}, false
	}
	return def.Recipe, true
}

func Render(id, serverID string, rawValues map[string]any) (Rendered, error) {
	def, ok := getDefinition(id)
	if !ok {
		return Rendered{}, fmt.Errorf("unknown recipe")
	}

	values, err := normalizeValues(def.Recipe, rawValues)
	if err != nil {
		return Rendered{}, err
	}

	rendered, err := renderContainer(def, serverID, values)
	if err != nil {
		return Rendered{}, err
	}
	return rendered, nil
}

func DesiredConfig(rendered Rendered) RenderedConfig {
	return RenderedConfig{
		RecipeID:      rendered.RecipeID,
		RecipeVersion: rendered.RecipeVersion,
		Values:        maps.Clone(rendered.Values),
		Rendered:      rendered,
	}
}

func getDefinition(id string) (definition, bool) {
	for _, def := range builtinDefinitions {
		if def.ID == id && def.Enabled {
			return def, true
		}
	}
	return definition{}, false
}

func mustLoadBuiltinDefinitions() []definition {
	entries, err := builtinRecipeFiles.ReadDir("builtin")
	if err != nil {
		panic(fmt.Sprintf("failed to read builtin recipes: %v", err))
	}

	result := []definition{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := builtinRecipeFiles.ReadFile("builtin/" + entry.Name())
		if err != nil {
			panic(fmt.Sprintf("failed to read builtin recipe %s: %v", entry.Name(), err))
		}

		var def definition
		if err := json.Unmarshal(data, &def); err != nil {
			panic(fmt.Sprintf("failed to parse builtin recipe %s: %v", entry.Name(), err))
		}
		if err := validateDefinition(def); err != nil {
			panic(fmt.Sprintf("invalid builtin recipe %s: %v", entry.Name(), err))
		}
		result = append(result, def)
	}

	slices.SortFunc(result, func(a, b definition) int {
		return strings.Compare(a.Name, b.Name)
	})
	return result
}

func validateDefinition(def definition) error {
	if strings.TrimSpace(def.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(def.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(def.Version) == "" {
		return fmt.Errorf("version is required")
	}
	if strings.TrimSpace(def.Container.Name) == "" {
		return fmt.Errorf("container.name is required")
	}
	if strings.TrimSpace(def.Container.Image) == "" {
		return fmt.Errorf("container.image is required")
	}

	keys := map[string]struct{}{}
	for _, input := range def.Inputs {
		if strings.TrimSpace(input.Key) == "" {
			return fmt.Errorf("input key is required")
		}
		if _, ok := keys[input.Key]; ok {
			return fmt.Errorf("duplicate input %q", input.Key)
		}
		keys[input.Key] = struct{}{}
	}

	return nil
}

func normalizeValues(recipe Recipe, rawValues map[string]any) (map[string]any, error) {
	if rawValues == nil {
		rawValues = map[string]any{}
	}

	known := map[string]Input{}
	for _, input := range recipe.Inputs {
		known[input.Key] = input
	}
	for key := range rawValues {
		if _, ok := known[key]; !ok {
			return nil, fmt.Errorf("unknown recipe input %q", key)
		}
	}

	values := map[string]any{}
	for _, input := range recipe.Inputs {
		value, ok := rawValues[input.Key]
		if !ok || value == nil || value == "" {
			value = input.Default
		}
		if input.Required && (value == nil || value == "") {
			return nil, fmt.Errorf("%s is required", input.Label)
		}

		normalized, err := normalizeValue(input, value)
		if err != nil {
			return nil, err
		}
		values[input.Key] = normalized
	}

	return values, nil
}

func normalizeValue(input Input, value any) (any, error) {
	switch input.Type {
	case InputString:
		text := strings.TrimSpace(fmt.Sprint(value))
		if input.Required && text == "" {
			return nil, fmt.Errorf("%s is required", input.Label)
		}
		if len(text) > 256 {
			return nil, fmt.Errorf("%s must be 256 characters or less", input.Label)
		}
		return text, nil
	case InputNumber:
		number, err := asInt(value)
		if err != nil {
			return nil, fmt.Errorf("%s must be a number", input.Label)
		}
		if input.Min != nil && number < *input.Min {
			return nil, fmt.Errorf("%s must be at least %d", input.Label, *input.Min)
		}
		if input.Max != nil && number > *input.Max {
			return nil, fmt.Errorf("%s must be at most %d", input.Label, *input.Max)
		}
		return number, nil
	case InputSelect:
		text := strings.TrimSpace(fmt.Sprint(value))
		if !slices.Contains(input.Options, text) {
			return nil, fmt.Errorf("%s must be one of: %s", input.Label, strings.Join(input.Options, ", "))
		}
		return text, nil
	default:
		return nil, fmt.Errorf("unsupported recipe input type %q", input.Type)
	}
}

func asInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if v != float64(int(v)) {
			return 0, fmt.Errorf("not an integer")
		}
		return int(v), nil
	case string:
		return strconv.Atoi(strings.TrimSpace(v))
	default:
		return 0, fmt.Errorf("not an integer")
	}
}

func renderContainer(def definition, serverID string, values map[string]any) (Rendered, error) {
	context := renderContext{ServerID: serverID, Values: values}

	name, err := renderString(def.Container.Name, context)
	if err != nil {
		return Rendered{}, err
	}
	image, err := renderString(def.Container.Image, context)
	if err != nil {
		return Rendered{}, err
	}

	rendered := Rendered{
		RecipeID:      def.ID,
		RecipeVersion: def.Version,
		Name:          name,
		Image:         image,
		Values:        maps.Clone(values),
		RestartPolicy: def.Container.RestartPolicy,
	}

	for _, port := range def.Container.Ports {
		hostPort, err := renderInt(port.HostPort, context)
		if err != nil {
			return Rendered{}, fmt.Errorf("host port: %w", err)
		}
		containerPort, err := renderInt(port.ContainerPort, context)
		if err != nil {
			return Rendered{}, fmt.Errorf("container port: %w", err)
		}
		rendered.Ports = append(rendered.Ports, PortMapping{
			HostPort:      hostPort,
			ContainerPort: containerPort,
			Protocol:      port.Protocol,
		})
	}

	for _, env := range def.Container.Env {
		value, err := renderString(env.Value, context)
		if err != nil {
			return Rendered{}, err
		}
		rendered.Env = append(rendered.Env, EnvVar{Key: env.Key, Value: value})
	}

	for _, volume := range def.Container.Volumes {
		hostPath, err := renderString(volume.HostPath, context)
		if err != nil {
			return Rendered{}, err
		}
		containerPath, err := renderString(volume.ContainerPath, context)
		if err != nil {
			return Rendered{}, err
		}
		rendered.Volumes = append(rendered.Volumes, VolumeMount{
			HostPath:      hostPath,
			ContainerPath: containerPath,
			ReadOnly:      volume.ReadOnly,
		})
	}

	return rendered, nil
}

type renderContext struct {
	ServerID string
	Values   map[string]any
}

func renderInt(value any, context renderContext) (int, error) {
	switch v := value.(type) {
	case float64:
		return asInt(v)
	case string:
		rendered, err := renderString(v, context)
		if err != nil {
			return 0, err
		}
		return asInt(rendered)
	default:
		return asInt(v)
	}
}

func renderString(template string, context renderContext) (string, error) {
	result := template
	for {
		start := strings.Index(result, "{{")
		if start == -1 {
			return result, nil
		}
		end := strings.Index(result[start+2:], "}}")
		if end == -1 {
			return "", fmt.Errorf("unterminated template in %q", template)
		}
		end += start + 2

		expression := strings.TrimSpace(result[start+2 : end])
		replacement, err := resolveTemplateExpression(expression, context)
		if err != nil {
			return "", err
		}
		result = result[:start] + replacement + result[end+2:]
	}
}

func resolveTemplateExpression(expression string, context renderContext) (string, error) {
	if expression == "server_id" {
		return context.ServerID, nil
	}
	if strings.HasPrefix(expression, "values.") {
		key := strings.TrimPrefix(expression, "values.")
		value, ok := context.Values[key]
		if !ok {
			return "", fmt.Errorf("unknown template value %q", key)
		}
		return fmt.Sprint(value), nil
	}
	return "", fmt.Errorf("unsupported template expression %q", expression)
}
