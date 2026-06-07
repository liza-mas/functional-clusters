package cluster

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	SchemaVersion = "functional-clusters.v1"
	RandomSeed    = 0

	scipSchema     = "scip.graph-export.v1"
	stacklitSchema = "stacklit.architecture-export.v1"

	weightingPolicy = "v1-default"
	dampingPolicy   = "v1-default"

	symbolPrefix    = "symbol:"
	packagePrefix   = "package:"
	componentPrefix = "component:"
)

type BuildOptions struct {
	SCIPGraphPath            string
	SCIPGraphPaths           []string
	StacklitArchitecturePath string
	RepositoryMetadataPath   string
	ADRMetadataPath          string
	GeneratedAt              time.Time
	GeneratorVersion         string
}

type Artifact struct {
	SchemaVersion string         `json:"schema_version"`
	Generator     Generator      `json:"generator"`
	GeneratedAt   string         `json:"generated_at"`
	Inputs        Inputs         `json:"inputs"`
	Clusters      []Cluster      `json:"clusters"`
	Relationships []Relationship `json:"relationships"`
	Unmapped      Unmapped       `json:"unmapped"`
	Diagnostics   []Diagnostic   `json:"diagnostics"`
}

type Generator struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	RandomSeed      int    `json:"random_seed"`
	WeightingPolicy string `json:"weighting_policy"`
	DampingPolicy   string `json:"damping_policy"`
}

type Inputs struct {
	SCIPGraph            RequiredInput  `json:"scip_graph"`
	StacklitArchitecture RequiredInput  `json:"stacklit_architecture"`
	RepositoryMetadata   *OptionalInput `json:"repository_metadata,omitempty"`
	ADRMetadata          *OptionalInput `json:"adr_metadata,omitempty"`
}

type RequiredInput struct {
	SchemaVersion string `json:"schema_version"`
	Fingerprint   string `json:"fingerprint"`
}

type OptionalInput struct {
	Fingerprint string `json:"fingerprint"`
}

type Cluster struct {
	ID                string            `json:"id"`
	Label             string            `json:"label"`
	Confidence        float64           `json:"confidence"`
	ConfidenceBand    string            `json:"confidence_band"`
	ConfidenceMetrics ConfidenceMetrics `json:"confidence_metrics"`
	Members           Members           `json:"members"`
	Summary           Summary           `json:"summary"`
}

type ConfidenceMetrics struct {
	Cohesion     float64 `json:"cohesion"`
	Separation   float64 `json:"separation"`
	LabelQuality float64 `json:"label_quality"`
}

type Members struct {
	Symbols    []string `json:"symbols"`
	Packages   []string `json:"packages"`
	Components []string `json:"components"`
}

type Summary struct {
	Purpose              string   `json:"purpose"`
	ImportantSymbols     []string `json:"important_symbols"`
	ImportantEntryPoints []string `json:"important_entry_points"`
}

type Relationship struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Type   string  `json:"type"`
	Weight float64 `json:"weight"`
}

type Unmapped struct {
	Symbols    []string `json:"symbols"`
	Packages   []string `json:"packages"`
	Components []string `json:"components"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type scipGraph struct {
	SchemaVersion string     `json:"schema_version"`
	Inputs        scipInputs `json:"inputs"`
	Nodes         []scipNode `json:"nodes"`
	Edges         []scipEdge `json:"edges"`
}

type scipInputs struct {
	SCIPIndex inputFingerprint `json:"scip_index"`
}

type inputFingerprint struct {
	Fingerprint string `json:"fingerprint"`
}

type scipNode struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name,omitempty"`
	Kind         string `json:"kind,omitempty"`
	Package      string `json:"package,omitempty"`
	DocumentPath string `json:"document_path,omitempty"`
}

type scipEdge struct {
	Source          string   `json:"source"`
	Target          string   `json:"target"`
	Type            string   `json:"type"`
	Provenance      string   `json:"provenance"`
	OccurrenceCount int      `json:"occurrence_count"`
	Weight          *float64 `json:"weight,omitempty"`
}

type architectureExport struct {
	SchemaVersion string             `json:"schema_version"`
	Inputs        architectureInputs `json:"inputs"`
	Packages      []architectureUnit `json:"packages"`
	Components    []architectureUnit `json:"components"`
	Membership    []membership       `json:"membership"`
	Relationships []architectureRel  `json:"relationships"`
	EntryPoints   []entryPoint       `json:"entry_points"`
}

type architectureInputs struct {
	StacklitIndex inputFingerprint `json:"stacklit_index"`
}

type architectureUnit struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type membership struct {
	Path      string `json:"path"`
	Package   string `json:"package"`
	Component string `json:"component"`
}

type architectureRel struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type entryPoint struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type repositoryMetadata struct {
	Name            string            `json:"repository_name"`
	DefaultBranch   string            `json:"default_branch"`
	LanguageSummary string            `json:"language_summary"`
	Fingerprint     string            `json:"fingerprint"`
	Extra           map[string]string `json:"-"`
}

type adrMetadata struct {
	Fingerprint string     `json:"fingerprint"`
	ADRs        []adrEntry `json:"adrs"`
}

type adrEntry struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Status         string   `json:"status"`
	RelatedPaths   []string `json:"related_package_paths"`
	ComponentPaths []string `json:"related_component_paths"`
}

type graphNode struct {
	id          string
	kind        string
	displayName string
	description string
	path        string
}

type weightedEdge struct {
	source string
	target string
	typ    string
	weight float64
	strong bool
}

type clusterDraft struct {
	key        string
	nodeIDs    []string
	symbols    []string
	packages   []string
	components []string
}

func BuildFromFiles(opts BuildOptions) (Artifact, error) {
	scip, err := readSCIPGraphs(opts.scipGraphPaths())
	if err != nil {
		return Artifact{}, err
	}
	archBytes, err := os.ReadFile(opts.StacklitArchitecturePath)
	if err != nil {
		return Artifact{}, fmt.Errorf("read Stacklit architecture: %w", err)
	}
	arch, err := decodeStacklitArchitecture(archBytes)
	if err != nil {
		return Artifact{}, err
	}
	return buildArtifact(scip, arch, opts)
}

func Build(scipBytes, architectureBytes []byte, opts BuildOptions) (Artifact, error) {
	scip, err := decodeSCIPGraph(scipBytes)
	if err != nil {
		return Artifact{}, err
	}
	arch, err := decodeStacklitArchitecture(architectureBytes)
	if err != nil {
		return Artifact{}, err
	}
	return buildArtifact(scip, arch, opts)
}

func buildArtifact(scip scipGraph, arch architectureExport, opts BuildOptions) (Artifact, error) {
	diagnostics, repositoryInput, adrInput, adrs := optionalMetadataDiagnostics(opts)
	nodes, edges, unmapped, entryPoints, units := buildUnifiedGraph(scip, arch, &diagnostics)
	drafts := detectCommunities(nodes, edges)
	clusters := materializeClusters(drafts, nodes, edges, entryPoints, units, adrs)
	relationships := rollupRelationships(clusters, edges)
	inputs := Inputs{
		SCIPGraph: RequiredInput{
			SchemaVersion: scip.SchemaVersion,
			Fingerprint:   scip.Inputs.SCIPIndex.Fingerprint,
		},
		StacklitArchitecture: RequiredInput{
			SchemaVersion: arch.SchemaVersion,
			Fingerprint:   arch.Inputs.StacklitIndex.Fingerprint,
		},
		RepositoryMetadata: repositoryInput,
		ADRMetadata:        adrInput,
	}

	return Artifact{
		SchemaVersion: SchemaVersion,
		Generator: Generator{
			Name:            "functional-clusters",
			Version:         opts.GeneratorVersion,
			RandomSeed:      RandomSeed,
			WeightingPolicy: weightingPolicy,
			DampingPolicy:   dampingPolicy,
		},
		GeneratedAt:   opts.GeneratedAt.UTC().Format(time.RFC3339),
		Inputs:        inputs,
		Clusters:      clusters,
		Relationships: relationships,
		Unmapped:      unmapped,
		Diagnostics:   sortDiagnostics(diagnostics),
	}, nil
}

func (opts BuildOptions) scipGraphPaths() []string {
	if len(opts.SCIPGraphPaths) > 0 {
		return append([]string(nil), opts.SCIPGraphPaths...)
	}
	if opts.SCIPGraphPath != "" {
		return []string{opts.SCIPGraphPath}
	}
	return nil
}

func readSCIPGraphs(paths []string) (scipGraph, error) {
	if len(paths) == 0 {
		return scipGraph{}, errors.New("missing required SCIP graph")
	}
	graphs := make([]scipGraph, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if len(paths) == 1 {
				return scipGraph{}, fmt.Errorf("read SCIP graph: %w", err)
			}
			return scipGraph{}, fmt.Errorf("read SCIP graph %q: %w", path, err)
		}
		graph, err := decodeSCIPGraph(data)
		if err != nil {
			if len(paths) == 1 {
				return scipGraph{}, err
			}
			return scipGraph{}, fmt.Errorf("SCIP graph %q: %w", path, err)
		}
		graphs = append(graphs, graph)
	}
	return mergeSCIPGraphs(graphs), nil
}

func decodeSCIPGraph(data []byte) (scipGraph, error) {
	var scip scipGraph
	if err := json.Unmarshal(data, &scip); err != nil {
		return scipGraph{}, fmt.Errorf("decode SCIP graph: %w", err)
	}
	if scip.SchemaVersion != scipSchema {
		return scipGraph{}, fmt.Errorf("unsupported SCIP graph schema_version %q", scip.SchemaVersion)
	}
	if scip.Inputs.SCIPIndex.Fingerprint == "" {
		return scipGraph{}, errors.New("SCIP graph missing inputs.scip_index.fingerprint")
	}
	return scip, nil
}

func decodeStacklitArchitecture(data []byte) (architectureExport, error) {
	var arch architectureExport
	if err := json.Unmarshal(data, &arch); err != nil {
		return architectureExport{}, fmt.Errorf("decode Stacklit architecture: %w", err)
	}
	if arch.SchemaVersion != stacklitSchema {
		return architectureExport{}, fmt.Errorf("unsupported Stacklit architecture schema_version %q", arch.SchemaVersion)
	}
	if arch.Inputs.StacklitIndex.Fingerprint == "" {
		return architectureExport{}, errors.New("Stacklit architecture missing inputs.stacklit_index.fingerprint")
	}
	return arch, nil
}

func mergeSCIPGraphs(graphs []scipGraph) scipGraph {
	if len(graphs) == 1 {
		return graphs[0]
	}

	nodeByID := map[string]scipNode{}
	edgeByKey := map[string]scipEdge{}
	fingerprints := make([]string, 0, len(graphs))
	for _, graph := range graphs {
		fingerprints = append(fingerprints, graph.Inputs.SCIPIndex.Fingerprint)
		for _, node := range graph.Nodes {
			existing, ok := nodeByID[node.ID]
			if !ok {
				nodeByID[node.ID] = node
				continue
			}
			nodeByID[node.ID] = mergeSCIPNode(existing, node)
		}
		for _, edge := range graph.Edges {
			edgeByKey[scipEdgeKey(edge)] = edge
		}
	}

	nodeIDs := keys(nodeByID)
	nodes := make([]scipNode, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		nodes = append(nodes, nodeByID[id])
	}

	edgeKeys := keys(edgeByKey)
	edges := make([]scipEdge, 0, len(edgeKeys))
	for _, key := range edgeKeys {
		edges = append(edges, edgeByKey[key])
	}

	return scipGraph{
		SchemaVersion: scipSchema,
		Inputs: scipInputs{SCIPIndex: inputFingerprint{
			Fingerprint: compositeSCIPFingerprint(fingerprints),
		}},
		Nodes: nodes,
		Edges: edges,
	}
}

func mergeSCIPNode(left, right scipNode) scipNode {
	return scipNode{
		ID:           left.ID,
		DisplayName:  chooseStableValue(left.DisplayName, right.DisplayName),
		Kind:         chooseStableValue(left.Kind, right.Kind),
		Package:      chooseStableValue(left.Package, right.Package),
		DocumentPath: chooseStableValue(left.DocumentPath, right.DocumentPath),
	}
}

func chooseStableValue(left, right string) string {
	switch {
	case left == "":
		return right
	case right == "":
		return left
	case right < left:
		return right
	default:
		return left
	}
}

func scipEdgeKey(edge scipEdge) string {
	weight := ""
	if edge.Weight != nil {
		weight = fmt.Sprintf("%g", *edge.Weight)
	}
	return strings.Join([]string{
		edge.Source,
		edge.Target,
		edge.Type,
		edge.Provenance,
		fmt.Sprintf("%d", edge.OccurrenceCount),
		weight,
	}, "\x00")
}

func compositeSCIPFingerprint(fingerprints []string) string {
	if len(fingerprints) == 1 {
		return fingerprints[0]
	}
	sorted := append([]string(nil), fingerprints...)
	sort.Strings(sorted)
	return Fingerprint([]byte(scipSchema + "\x00" + strings.Join(sorted, "\x00")))
}

func WriteArtifact(path string, artifact Artifact) error {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal artifact: %w", err)
	}
	data = append(data, '\n')
	if path == "" || path == "-" {
		_, err = os.Stdout.Write(data)
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".functional-clusters-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp output: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp output: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp output: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace output: %w", err)
	}
	return nil
}

func ReadArtifact(path string) (Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Artifact{}, err
	}
	var artifact Artifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return Artifact{}, err
	}
	if artifact.SchemaVersion != SchemaVersion {
		return Artifact{}, fmt.Errorf("unsupported cluster artifact schema_version %q", artifact.SchemaVersion)
	}
	return artifact, nil
}

func RenderList(artifact Artifact, w io.Writer) error {
	_, err := fmt.Fprintln(w, "ID\tLABEL\tCONFIDENCE\tBAND")
	if err != nil {
		return err
	}
	for _, cluster := range artifact.Clusters {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%.2f\t%s\n", cluster.ID, cluster.Label, cluster.Confidence, cluster.ConfidenceBand); err != nil {
			return err
		}
	}
	return nil
}

func RenderExplain(artifact Artifact, symbol string, w io.Writer) error {
	var found *Cluster
	for i := range artifact.Clusters {
		if contains(artifact.Clusters[i].Members.Symbols, symbol) {
			found = &artifact.Clusters[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("symbol not found in cluster artifact: %s", symbol)
	}
	_, err := fmt.Fprintf(w, "Symbol: %s\nCluster: %s\t%s\t%.2f\t%s\nPurpose: %s\n", symbol, found.ID, found.Label, found.Confidence, found.ConfidenceBand, found.Summary.Purpose)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Nearby clusters:"); err != nil {
		return err
	}
	for _, rel := range artifact.Relationships {
		if rel.Source == found.ID || rel.Target == found.ID {
			if _, err := fmt.Fprintf(w, "- %s -> %s %.2f\n", rel.Source, rel.Target, rel.Weight); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintln(w, "Dependency boundaries:"); err != nil {
		return err
	}
	for _, rel := range artifact.Relationships {
		if rel.Source == found.ID {
			if _, err := fmt.Fprintf(w, "- outgoing %s %.2f\n", rel.Target, rel.Weight); err != nil {
				return err
			}
		}
		if rel.Target == found.ID {
			if _, err := fmt.Fprintf(w, "- incoming %s %.2f\n", rel.Source, rel.Weight); err != nil {
				return err
			}
		}
	}
	return nil
}

func optionalMetadataDiagnostics(opts BuildOptions) ([]Diagnostic, *OptionalInput, *OptionalInput, *adrMetadata) {
	var diagnostics []Diagnostic
	addOptional := func(path, code, label string, target any) *OptionalInput {
		if path == "" {
			diagnostics = append(diagnostics, Diagnostic{Severity: "info", Code: code + "_absent", Message: label + " metadata not supplied"})
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{Severity: "warning", Code: code + "_unreadable", Message: label + " metadata unreadable; ignored"})
			return nil
		}
		if err := json.Unmarshal(data, target); err != nil {
			diagnostics = append(diagnostics, Diagnostic{Severity: "warning", Code: code + "_malformed", Message: label + " metadata malformed; ignored"})
			return nil
		}
		return &OptionalInput{Fingerprint: Fingerprint(data)}
	}
	var repo repositoryMetadata
	var adrs adrMetadata
	repositoryInput := addOptional(opts.RepositoryMetadataPath, "repository_metadata", "repository", &repo)
	adrInput := addOptional(opts.ADRMetadataPath, "adr_metadata", "ADR", &adrs)
	if adrInput == nil {
		return diagnostics, repositoryInput, adrInput, nil
	}
	return diagnostics, repositoryInput, adrInput, &adrs
}

func buildUnifiedGraph(scip scipGraph, arch architectureExport, diagnostics *[]Diagnostic) (map[string]graphNode, []weightedEdge, Unmapped, map[string][]string, map[string]architectureUnit) {
	nodes := map[string]graphNode{}
	units := map[string]architectureUnit{}
	pathMembership := map[string]membership{}
	packageHasSymbol := map[string]bool{}
	componentHasSymbol := map[string]bool{}
	entryPoints := map[string][]string{}

	for _, pkg := range arch.Packages {
		id := packagePrefix + pkg.ID
		nodes[id] = graphNode{id: id, kind: "package", displayName: pkg.Name, description: pkg.Description}
		units[id] = pkg
	}
	for _, comp := range arch.Components {
		id := componentPrefix + comp.ID
		nodes[id] = graphNode{id: id, kind: "component", displayName: comp.Name, description: comp.Description}
		units[id] = comp
	}
	for _, mem := range arch.Membership {
		pathMembership[normalizePath(mem.Path)] = mem
	}
	for _, ep := range arch.EntryPoints {
		entryPoints[normalizePath(ep.Path)] = append(entryPoints[normalizePath(ep.Path)], ep.Path)
	}

	var edges []weightedEdge
	for _, pkg := range arch.Packages {
		if _, ok := nodes[componentPrefix+pkg.ID]; ok {
			edges = append(edges, weightedEdge{source: packagePrefix + pkg.ID, target: componentPrefix + pkg.ID, typ: "architecture_identity", weight: 3, strong: true})
		}
	}
	for _, node := range scip.Nodes {
		id := symbolPrefix + node.ID
		path := normalizePath(node.DocumentPath)
		nodes[id] = graphNode{id: id, kind: "symbol", displayName: firstNonEmpty(node.DisplayName, displayNameFromSymbol(node.ID)), path: path}
		if mem, ok := pathMembership[path]; ok {
			if mem.Package != "" {
				packageHasSymbol[mem.Package] = true
				edges = append(edges, weightedEdge{source: id, target: packagePrefix + mem.Package, typ: "membership", weight: 3, strong: true})
			}
			if mem.Component != "" {
				componentHasSymbol[mem.Component] = true
				edges = append(edges, weightedEdge{source: id, target: componentPrefix + mem.Component, typ: "membership", weight: 3, strong: true})
			}
		}
	}

	degree := map[string]float64{}
	for _, edge := range scip.Edges {
		edgeType, w, ok := edgeWeight(edge)
		if !ok {
			*diagnostics = appendUnknownEdgeDiagnostic(*diagnostics, edge.Type, edge.Provenance)
			continue
		}
		_ = edgeType
		source := symbolPrefix + edge.Source
		target := symbolPrefix + edge.Target
		degree[source] += w
		degree[target] += w
	}

	for _, edge := range scip.Edges {
		edgeType, w, ok := edgeWeight(edge)
		if !ok {
			continue
		}
		source := symbolPrefix + edge.Source
		target := symbolPrefix + edge.Target
		strong := edgeType == "implementation" || edgeType == "contained_dependency"
		if edgeType == "reference" && degree[source] < 4 && degree[target] < 4 {
			strong = true
		}
		if degree[source] >= 4 || degree[target] >= 4 {
			w = math.Min(w, 0.5)
			strong = false
		}
		edges = append(edges, weightedEdge{source: source, target: target, typ: edgeType, weight: w, strong: strong})
	}

	for _, rel := range arch.Relationships {
		source := packagePrefix + rel.Source
		target := packagePrefix + rel.Target
		if rel.Type == "component_dependency" {
			source = componentPrefix + rel.Source
			target = componentPrefix + rel.Target
		}
		edges = append(edges, weightedEdge{source: source, target: target, typ: rel.Type, weight: 1, strong: false})
	}

	var unmapped Unmapped
	for _, node := range scip.Nodes {
		if _, ok := pathMembership[normalizePath(node.DocumentPath)]; !ok {
			unmapped.Symbols = append(unmapped.Symbols, node.ID)
		}
	}
	for _, pkg := range arch.Packages {
		if !packageHasSymbol[pkg.ID] {
			unmapped.Packages = append(unmapped.Packages, pkg.ID)
		}
	}
	for _, comp := range arch.Components {
		if !componentHasSymbol[comp.ID] {
			unmapped.Components = append(unmapped.Components, comp.ID)
		}
	}
	sort.Strings(unmapped.Symbols)
	sort.Strings(unmapped.Packages)
	sort.Strings(unmapped.Components)
	return nodes, edges, unmapped, entryPoints, units
}

func detectCommunities(nodes map[string]graphNode, edges []weightedEdge) []clusterDraft {
	ds := newDisjointSet(keys(nodes))
	strongDegree := map[string]int{}
	for _, edge := range edges {
		if edge.strong {
			ds.union(edge.source, edge.target)
			strongDegree[edge.source]++
			strongDegree[edge.target]++
		}
	}
	groups := map[string][]string{}
	for _, id := range keys(nodes) {
		groups[ds.find(id)] = append(groups[ds.find(id)], id)
	}
	attachWeakOnlyNodes(groups, edges, strongDegree, nodes)
	var drafts []clusterDraft
	for _, ids := range groups {
		sort.Strings(ids)
		drafts = append(drafts, draftFromNodes(ids, nodes))
	}
	sort.Slice(drafts, func(i, j int) bool { return drafts[i].key < drafts[j].key })
	return drafts
}

func attachWeakOnlyNodes(groups map[string][]string, edges []weightedEdge, strongDegree map[string]int, nodes map[string]graphNode) {
	nodeGroup := map[string]string{}
	for group, ids := range groups {
		for _, id := range ids {
			nodeGroup[id] = group
		}
	}
	stableTargets := map[string]bool{}
	var weakNodes []string
	weakNodeGroup := map[string]string{}
	for _, group := range keys(groups) {
		ids := groups[group]
		if len(ids) == 1 && strongDegree[ids[0]] == 0 {
			weakNodes = append(weakNodes, ids[0])
			weakNodeGroup[ids[0]] = group
			continue
		}
		stableTargets[group] = true
	}
	sort.Strings(weakNodes)

	moves := map[string]string{}
	for _, node := range weakNodes {
		scores := map[string]float64{}
		for _, edge := range edges {
			switch {
			case edge.source == node && stableTargets[nodeGroup[edge.target]]:
				scores[nodeGroup[edge.target]] += edge.weight
			case edge.target == node && stableTargets[nodeGroup[edge.source]]:
				scores[nodeGroup[edge.source]] += edge.weight
			}
		}
		if len(scores) == 0 {
			continue
		}
		bestGroup := ""
		bestScore := -1.0
		for candidate, score := range scores {
			if score > bestScore || (score == bestScore && groupKey(groups[candidate], nodes) < groupKey(groups[bestGroup], nodes)) {
				bestGroup = candidate
				bestScore = score
			}
		}
		moves[node] = bestGroup
	}

	for _, node := range weakNodes {
		target := moves[node]
		if target == "" {
			continue
		}
		source := weakNodeGroup[node]
		groups[target] = append(groups[target], node)
		sort.Strings(groups[target])
		delete(groups, source)
	}
}

func groupKey(ids []string, nodes map[string]graphNode) string {
	if len(ids) == 0 {
		return ""
	}
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	return draftFromNodes(sorted, nodes).key
}

func materializeClusters(drafts []clusterDraft, nodes map[string]graphNode, edges []weightedEdge, entryPoints map[string][]string, units map[string]architectureUnit, adrs *adrMetadata) []Cluster {
	var clusters []Cluster
	for i, draft := range drafts {
		id := fmt.Sprintf("cluster-%03d", i+1)
		label, labelQuality := labelForDraft(draft, nodes, units, adrs)
		metrics := confidenceForDraft(draft, edges, labelQuality)
		if len(drafts) == 1 {
			metrics.Separation = 0
		}
		confidence := round2((metrics.Cohesion + metrics.Separation + metrics.LabelQuality) / 3)
		clusters = append(clusters, Cluster{
			ID:             id,
			Label:          label,
			Confidence:     confidence,
			ConfidenceBand: confidenceBand(confidence),
			ConfidenceMetrics: ConfidenceMetrics{
				Cohesion:     round2(metrics.Cohesion),
				Separation:   round2(metrics.Separation),
				LabelQuality: round2(metrics.LabelQuality),
			},
			Members: Members{
				Symbols:    trimPrefixSorted(draft.symbols, symbolPrefix),
				Packages:   trimPrefixSorted(draft.packages, packagePrefix),
				Components: trimPrefixSorted(draft.components, componentPrefix),
			},
			Summary: summaryForDraft(label, draft, nodes, entryPoints),
		})
	}
	return clusters
}

func rollupRelationships(clusters []Cluster, edges []weightedEdge) []Relationship {
	nodeCluster := map[string]string{}
	for _, cluster := range clusters {
		for _, symbol := range cluster.Members.Symbols {
			nodeCluster[symbolPrefix+symbol] = cluster.ID
		}
		for _, pkg := range cluster.Members.Packages {
			nodeCluster[packagePrefix+pkg] = cluster.ID
		}
		for _, comp := range cluster.Members.Components {
			nodeCluster[componentPrefix+comp] = cluster.ID
		}
	}
	weights := map[string]float64{}
	for _, edge := range edges {
		source := nodeCluster[edge.source]
		target := nodeCluster[edge.target]
		if source == "" || target == "" || source == target {
			continue
		}
		key := source + "\x00" + target + "\x00dependency"
		weights[key] += edge.weight
	}
	keys := keysFloat(weights)
	var relationships []Relationship
	for _, key := range keys {
		parts := strings.Split(key, "\x00")
		relationships = append(relationships, Relationship{Source: parts[0], Target: parts[1], Type: parts[2], Weight: round2(weights[key])})
	}
	return relationships
}

type disjointSet struct {
	parent map[string]string
}

func newDisjointSet(ids []string) *disjointSet {
	parent := map[string]string{}
	for _, id := range ids {
		parent[id] = id
	}
	return &disjointSet{parent: parent}
}

func (d *disjointSet) find(id string) string {
	if d.parent[id] != id {
		d.parent[id] = d.find(d.parent[id])
	}
	return d.parent[id]
}

func (d *disjointSet) union(a, b string) {
	ra := d.find(a)
	rb := d.find(b)
	if ra == rb {
		return
	}
	if ra < rb {
		d.parent[rb] = ra
	} else {
		d.parent[ra] = rb
	}
}

func draftFromNodes(ids []string, nodes map[string]graphNode) clusterDraft {
	draft := clusterDraft{nodeIDs: ids}
	for _, id := range ids {
		switch {
		case strings.HasPrefix(id, symbolPrefix):
			draft.symbols = append(draft.symbols, id)
		case strings.HasPrefix(id, packagePrefix):
			draft.packages = append(draft.packages, id)
		case strings.HasPrefix(id, componentPrefix):
			draft.components = append(draft.components, id)
		}
	}
	sort.Strings(draft.symbols)
	sort.Strings(draft.packages)
	sort.Strings(draft.components)
	switch {
	case len(draft.symbols) > 0:
		draft.key = draft.symbols[0]
	case len(draft.packages) > 0:
		draft.key = draft.packages[0]
	case len(draft.components) > 0:
		draft.key = draft.components[0]
	default:
		draft.key = ids[0]
	}
	_ = nodes
	return draft
}

func labelForDraft(draft clusterDraft, nodes map[string]graphNode, units map[string]architectureUnit, adrs *adrMetadata) (string, float64) {
	if title := adrTitleForDraft(draft, adrs); title != "" {
		return title, 1.0
	}
	if len(draft.components) > 0 {
		name := nodes[draft.components[0]].displayName
		if name != "" {
			return name, 0.82
		}
	}
	if len(draft.packages) > 0 {
		name := nodes[draft.packages[0]].displayName
		if name != "" {
			return name, 0.72
		}
	}
	if len(draft.symbols) > 0 {
		name := nodes[draft.symbols[0]].displayName
		if name != "" {
			return name, 0.55
		}
	}
	_ = units
	return "Unlabeled", 0.2
}

func adrTitleForDraft(draft clusterDraft, adrs *adrMetadata) string {
	if adrs == nil {
		return ""
	}
	paths := map[string]bool{}
	for _, pkg := range trimPrefixSorted(draft.packages, packagePrefix) {
		paths[normalizePath(pkg)] = true
	}
	for _, component := range trimPrefixSorted(draft.components, componentPrefix) {
		paths[normalizePath(component)] = true
	}
	var titles []string
	for _, adr := range adrs.ADRs {
		for _, related := range append(append([]string{}, adr.RelatedPaths...), adr.ComponentPaths...) {
			if paths[normalizePath(related)] && adr.Title != "" {
				titles = append(titles, adr.Title)
				break
			}
		}
	}
	sort.Strings(titles)
	if len(titles) == 0 {
		return ""
	}
	return titles[0]
}

func summaryForDraft(label string, draft clusterDraft, nodes map[string]graphNode, entryPoints map[string][]string) Summary {
	purpose := "Groups repository functionality around " + label + "."
	for _, id := range append(append([]string{}, draft.components...), draft.packages...) {
		if nodes[id].description != "" {
			purpose = nodes[id].description
			break
		}
	}
	var important []string
	for _, symbol := range draft.symbols {
		important = append(important, strings.TrimPrefix(symbol, symbolPrefix))
		if len(important) == 5 {
			break
		}
	}
	var eps []string
	seen := map[string]bool{}
	for _, symbol := range draft.symbols {
		for _, ep := range entryPoints[nodes[symbol].path] {
			if !seen[ep] {
				seen[ep] = true
				eps = append(eps, ep)
			}
		}
	}
	sort.Strings(eps)
	return Summary{Purpose: purpose, ImportantSymbols: important, ImportantEntryPoints: eps}
}

func confidenceForDraft(draft clusterDraft, edges []weightedEdge, labelQuality float64) ConfidenceMetrics {
	member := map[string]bool{}
	for _, id := range draft.nodeIDs {
		member[id] = true
	}
	var internal, external float64
	for _, edge := range edges {
		source := member[edge.source]
		target := member[edge.target]
		switch {
		case source && target:
			internal += edge.weight
		case source || target:
			external += edge.weight
		}
	}
	cohesion := 0.0
	if len(draft.nodeIDs) <= 1 {
		cohesion = 0.3
	} else {
		cohesion = clamp(internal / float64(len(draft.nodeIDs)*3))
	}
	separation := 1.0
	if internal+external > 0 {
		separation = clamp(internal / (internal + external))
	}
	return ConfidenceMetrics{Cohesion: cohesion, Separation: separation, LabelQuality: labelQuality}
}

func edgeWeight(edge scipEdge) (string, float64, bool) {
	edgeType := edge.Type
	if edgeType == "dependency" && edge.Provenance == "contained_dependency" {
		edgeType = "contained_dependency"
	}
	switch edgeType {
	case "implementation":
		return edgeType, 3, true
	case "contained_dependency":
		return edgeType, 2, true
	case "reference":
		return edgeType, 1, true
	default:
		return edgeType, 0, false
	}
}

func appendUnknownEdgeDiagnostic(diagnostics []Diagnostic, edgeType, provenance string) []Diagnostic {
	code := "unknown_edge_type"
	message := "ignored unknown SCIP edge type " + edgeType + " with provenance " + provenance
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code && diagnostic.Message == message {
			return diagnostics
		}
	}
	return append(diagnostics, Diagnostic{Severity: "warning", Code: code, Message: message})
}

func confidenceBand(confidence float64) string {
	switch {
	case confidence >= 0.75:
		return "high"
	case confidence >= 0.50:
		return "medium"
	default:
		return "low"
	}
}

func normalizePath(path string) string {
	if path == "" {
		return ""
	}
	cleaned := filepath.ToSlash(filepath.Clean(path))
	return strings.TrimPrefix(cleaned, "./")
}

func displayNameFromSymbol(symbol string) string {
	symbol = strings.TrimSuffix(symbol, ".")
	idx := strings.LastIndexAny(symbol, "`/.")
	if idx >= 0 && idx+1 < len(symbol) {
		return symbol[idx+1:]
	}
	return symbol
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func trimPrefixSorted(values []string, prefix string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strings.TrimPrefix(value, prefix))
	}
	sort.Strings(out)
	return out
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func keysFloat(m map[string]float64) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func sortDiagnostics(diagnostics []Diagnostic) []Diagnostic {
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].Severity != diagnostics[j].Severity {
			return diagnostics[i].Severity < diagnostics[j].Severity
		}
		if diagnostics[i].Code != diagnostics[j].Code {
			return diagnostics[i].Code < diagnostics[j].Code
		}
		return diagnostics[i].Message < diagnostics[j].Message
	})
	return diagnostics
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func Fingerprint(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", sum)
}
