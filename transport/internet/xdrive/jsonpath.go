package xdrive

import (
	"encoding/json"
	"strings"
)

func jsonWalk(payload []byte, path string) any {
	var root any
	if json.Unmarshal(payload, &root) != nil {
		return nil
	}
	node := root
	for key := range strings.SplitSeq(path, ".") {
		obj, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		node, ok = obj[key]
		if !ok {
			return nil
		}
	}
	return node
}

func jsonString(payload []byte, path string) string {
	if s, ok := jsonWalk(payload, path).(string); ok {
		return s
	}
	return ""
}

func jsonNumber(payload []byte, path string) int64 {
	if f, ok := jsonWalk(payload, path).(float64); ok {
		return int64(f)
	}
	return 0
}
