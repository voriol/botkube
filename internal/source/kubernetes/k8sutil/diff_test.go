package k8sutil_test

import (
	"fmt"
	"testing"

	"github.com/MakeNowJust/heredoc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kubeshop/botkube/internal/source/kubernetes/config"
	"github.com/kubeshop/botkube/internal/source/kubernetes/k8sutil"
)

// Object mocks kubernetes objects
type Object struct {
	Spec   Spec   `json:"spec"`
	Status Status `json:"status"`
	Data   Data   `json:"data"`
	Rules  Rules  `json:"rules"`
	Other  Other  `json:"other"`
}

// Other mocks fields like MetaData, Status etc in kubernetes objects
type Other struct {
	Foo         string            `json:"foo"`
	Annotations map[string]string `json:"annotations"`
}

// Spec mocks ObjectSpec field in kubernetes object
type Spec struct {
	Port       int         `json:"port"`
	Containers []Container `json:"containers"`
}

// Container mocks ObjectSpec.Container field in kubernetes object
type Container struct {
	Image string `json:"image"`
}

// Status mocks ObjectStatus field in kubernetes object
type Status struct {
	Replicas int `json:"replicas"`
}

// Data mocks ObjectData field in kubernetes object like configmap
type Data struct {
	Properties string `json:"properties"`
}

// Rules mocks ObjectRules field in kubernetes object
type Rules struct {
	Verbs string `json:"verbs"`
}

// ExpectedDiff struct to generate expected diff
type ExpectedDiff struct {
	Path string
	X    string
	Y    string
}

func TestDiff(t *testing.T) {
	tests := map[string]struct {
		old                Object
		new                Object
		update             config.UpdateSetting
		expected           ExpectedDiff
		expectedErrMessage string
	}{
		`Spec Diff`: {
			old:    Object{Spec: Spec{Containers: []Container{{Image: "nginx:1.14"}}}, Other: Other{Foo: "bar"}},
			new:    Object{Spec: Spec{Containers: []Container{{Image: "nginx:latest"}}}, Other: Other{Foo: "bar"}},
			update: config.UpdateSetting{Fields: []string{"spec.containers[*].image"}, IncludeDiff: true},
			expected: ExpectedDiff{
				Path: "spec.containers[*].image",
				X:    "nginx:1.14",
				Y:    "nginx:latest",
			},
		},
		`Non Spec Diff`: {
			old:      Object{Spec: Spec{Containers: []Container{{Image: "nginx:1.14"}}}, Other: Other{Foo: "bar"}},
			new:      Object{Spec: Spec{Containers: []Container{{Image: "nginx:1.14"}}}, Other: Other{Foo: "boo"}},
			update:   config.UpdateSetting{Fields: []string{"metadata.name"}, IncludeDiff: true},
			expected: ExpectedDiff{},
		},
		`Annotations changed`: {
			old:    Object{Other: Other{Annotations: map[string]string{"app.kubernetes.io/version": "1"}}},
			new:    Object{Other: Other{Annotations: map[string]string{"app.kubernetes.io/version": "2"}}},
			update: config.UpdateSetting{Fields: []string{`other.annotations.app\.kubernetes\.io\/version`}, IncludeDiff: true},
			expected: ExpectedDiff{
				Path: `other.annotations.app\.kubernetes\.io\/version`,
				X:    "1",
				Y:    "2",
			},
		},
		`Status Diff`: {
			old:    Object{Status: Status{Replicas: 1}, Other: Other{Foo: "bar"}},
			new:    Object{Status: Status{Replicas: 2}, Other: Other{Foo: "bar"}},
			update: config.UpdateSetting{Fields: []string{"status.replicas"}, IncludeDiff: true},
			expected: ExpectedDiff{
				Path: "status.replicas",
				X:    "1",
				Y:    "2",
			},
		},
		`Non Status Diff`: {
			old:      Object{Status: Status{Replicas: 1}, Other: Other{Foo: "bar"}},
			new:      Object{Status: Status{Replicas: 1}, Other: Other{Foo: "boo"}},
			update:   config.UpdateSetting{Fields: []string{"metadata.labels"}, IncludeDiff: true},
			expected: ExpectedDiff{},
		},
		`Event Diff`: {
			old:    Object{Data: Data{Properties: "color: blue"}, Other: Other{Foo: "bar"}},
			new:    Object{Data: Data{Properties: "color: red"}, Other: Other{Foo: "bar"}},
			update: config.UpdateSetting{Fields: []string{"data.properties"}, IncludeDiff: true},
			expected: ExpectedDiff{
				Path: "data.properties",
				X:    "color: blue",
				Y:    "color: red",
			},
		},
		`Non Event Diff`: {
			old:      Object{Data: Data{Properties: "color: blue"}, Other: Other{Foo: "bar"}},
			new:      Object{Data: Data{Properties: "color: blue"}, Other: Other{Foo: "boo"}},
			update:   config.UpdateSetting{Fields: []string{"metadata.name"}, IncludeDiff: true},
			expected: ExpectedDiff{},
		},
		`Rules Diff`: {
			old:    Object{Rules: Rules{Verbs: "list"}, Other: Other{Foo: "bar"}},
			new:    Object{Rules: Rules{Verbs: "watch"}, Other: Other{Foo: "bar"}},
			update: config.UpdateSetting{Fields: []string{"rules.verbs"}, IncludeDiff: true},
			expected: ExpectedDiff{
				Path: "rules.verbs",
				X:    "list",
				Y:    "watch",
			},
		},
		`JSONPath error`: {
			old:    Object{Rules: Rules{Verbs: "list"}, Other: Other{Foo: "bar"}},
			new:    Object{Rules: Rules{Verbs: "list"}, Other: Other{Foo: "boo"}},
			update: config.UpdateSetting{Fields: []string{"><@>!$@435metadata.name"}, IncludeDiff: true},
			expectedErrMessage: heredoc.Doc(`
				while getting diff: 1 error occurred:
					* while finding value in old obj from jsonpath "><@>!$@435metadata.name": unrecognized character in action: U+003E '>'`),
		},
		`Non Rules Diff`: {
			old:      Object{Rules: Rules{Verbs: "list"}, Other: Other{Foo: "bar"}},
			new:      Object{Rules: Rules{Verbs: "list"}, Other: Other{Foo: "boo"}},
			update:   config.UpdateSetting{Fields: []string{"metadata.name"}, IncludeDiff: true},
			expected: ExpectedDiff{},
		},
		`Get all diffs even if one of them return errors`: {
			old:    Object{Status: Status{Replicas: 1}, Other: Other{Foo: "bar"}},
			new:    Object{Status: Status{Replicas: 2}, Other: Other{Foo: "bar"}},
			update: config.UpdateSetting{Fields: []string{"status.foo", "status.replicas"}, IncludeDiff: true},
			expected: ExpectedDiff{
				Path: "status.replicas",
				X:    "1",
				Y:    "2",
			},
		},
		`Missing Property in old object`: {
			old:    Object{Status: Status{Replicas: 1}, Other: Other{Annotations: nil}},
			new:    Object{Status: Status{Replicas: 2}, Other: Other{Annotations: map[string]string{"foo": "bar"}}},
			update: config.UpdateSetting{Fields: []string{"other.annotations.foo"}, IncludeDiff: true},
			expected: ExpectedDiff{
				Path: "other.annotations.foo",
				X:    "<none>",
				Y:    "bar",
			},
		},
		`Missing Property in new object`: {
			old:    Object{Status: Status{Replicas: 1}, Other: Other{Annotations: map[string]string{"foo": "bar"}}},
			new:    Object{Status: Status{Replicas: 2}, Other: Other{Annotations: nil}},
			update: config.UpdateSetting{Fields: []string{"other.annotations.foo"}, IncludeDiff: true},
			expected: ExpectedDiff{
				Path: "other.annotations.foo",
				X:    "bar",
				Y:    "<none>",
			},
		},
	}
	for name, test := range tests {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			actual, err := k8sutil.Diff(test.old, test.new, test.update)

			if test.expectedErrMessage != "" {
				require.Error(t, err)
				assert.Equal(t, test.expectedErrMessage, err.Error())
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected.MockDiff(), actual)
		})
	}
}

// MockDiff mocks diff.Diff
func (e *ExpectedDiff) MockDiff() string {
	if e.Path == "" {
		return ""
	}
	return fmt.Sprintf("%+v:\n\t-: %+v\n\t+: %+v\n", e.Path, e.X, e.Y)
}
