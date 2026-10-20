package server

import "fmt"

// labelPatch is the label part of an update tool: replace every label, merge
// single keys into the current set, or remove keys from it. One mode per call,
// because a caller that sends two would not be able to predict the result.
type labelPatch struct {
	Labels          map[string]string `json:"labels,omitempty" jsonschema:"Replacement label map. Omit to keep the current labels. An empty object clears every label."`
	AddLabels       map[string]string `json:"add_labels,omitempty" jsonschema:"Labels to add or overwrite. Every other label is preserved."`
	RemoveLabelKeys []string          `json:"remove_label_keys,omitempty" jsonschema:"Label keys to remove. Every other label is preserved."`
}

// apply returns the map to send and whether the caller asked to change labels
// at all. false means leave the resource labels untouched.
func (p labelPatch) apply(current map[string]string) (map[string]string, bool, error) {
	modes := 0
	if p.Labels != nil {
		modes++
	}
	if len(p.AddLabels) > 0 {
		modes++
	}
	if len(p.RemoveLabelKeys) > 0 {
		modes++
	}
	if modes > 1 {
		return nil, false, fmt.Errorf("choose one of labels, add_labels, or remove_label_keys, not more than one")
	}
	if p.Labels != nil {
		return p.Labels, true, nil
	}
	if modes == 0 {
		return nil, false, nil
	}
	out := make(map[string]string, len(current)+len(p.AddLabels))
	for key, value := range current {
		out[key] = value
	}
	for key, value := range p.AddLabels {
		if key == "" {
			return nil, false, fmt.Errorf("add_labels: a key is empty")
		}
		out[key] = value
	}
	for _, key := range p.RemoveLabelKeys {
		delete(out, key)
	}
	return out, true, nil
}

func (p labelPatch) empty() bool {
	return p.Labels == nil && len(p.AddLabels) == 0 && len(p.RemoveLabelKeys) == 0
}
