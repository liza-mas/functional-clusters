package cli

import (
	"encoding/json"

	"github.com/liza-mas/functional-clusters/internal/cluster"
)

func clusterJSON(artifact cluster.Artifact) ([]byte, error) {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
