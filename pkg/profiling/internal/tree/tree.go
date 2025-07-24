package tree

import (
	"fmt"
	"sort" // Keep sort import for potential future use
	"time"

	"github.com/google/pprof/profile"
)

type FlameGraphNode struct {
	Name             string            `json:"name"`
	Value            int64             `json:"value"`
	Children         []*FlameGraphNode `json:"children,omitempty"`
	SelfValue        int64             `json:"selfValue,omitempty"`
	ValueFormatted   string            `json:"valueFormatted,omitempty"`
	FilePath         string            `json:"filePath,omitempty"`
	LineNum          int               `json:"lineNum,omitempty"`
	ObjectCount      int64             `json:"objectCount,omitempty"`
	AvgSize          int64             `json:"avgSize,omitempty"`
	AvgSizeFormatted string            `json:"avgSizeFormatted,omitempty"`
	Type             string            `json:"type,omitempty"`
}

// nodeKey uniquely identifies a node in the call tree based on function ID.
// Using only function ID aggregates all calls to the same function regardless of call site,
// which is typical for basic flame graphs. More complex keys could include location ID.
type nodeKey struct {
	funcID uint64
}

// tempNode is used during the tree construction process.
type tempNode struct {
	node        *FlameGraphNode
	children    map[nodeKey]*tempNode
	selfValue   int64  // Value attributed directly to this node (leaf of a sample stack)
	objectCount int64  // Object count for memory profiles
	filePath    string // Source file path
	lineNum     int    // Source line number
	objectType  string // Object type for memory profiles
}

func FormatSampleValue(value int64, unit string) string {
	switch unit {
	case "nanoseconds":
		d := time.Duration(value) * time.Nanosecond
		if d >= time.Second {
			return fmt.Sprintf("%.2fs", d.Seconds())
		}
		if d >= time.Millisecond {
			return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
		}
		if d >= time.Microsecond {
			return fmt.Sprintf("%.2fus", float64(d.Microseconds()))
		}
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case "count":
		return fmt.Sprintf("%d", value)
	// 如果需要，可以添加其他潜在单位的处理
	default:
		return fmt.Sprintf("%d %s", value, unit) // 回退方案
	}
}

func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp]) // Kilo, Mega, Giga, Tera, Peta, Exa
}

func SearchNodeByName(node *FlameGraphNode, name string) *FlameGraphNode {
	if node.Name == name {
		return node
	}
	for _, child := range node.Children {
		if result := SearchNodeByName(child, name); result != nil {
			return result
		}
	}
	return nil
}

func FindTopNNodes(node *FlameGraphNode, n int) []*FlameGraphNode {
	var allNodes []*FlameGraphNode
	var collectNodes func(n *FlameGraphNode)
	collectNodes = func(n *FlameGraphNode) {
		allNodes = append(allNodes, n)
		for _, child := range n.Children {
			collectNodes(child)
		}
	}
	collectNodes(node)

	sort.Slice(allNodes, func(i, j int) bool {
		return allNodes[i].Value > allNodes[j].Value
	})

	if n > len(allNodes) {
		n = len(allNodes)
	}
	return allNodes[:n]
}

// BuildFlameGraphTree converts pprof profile data into a hierarchical FlameGraphNode structure.
// valueIndex specifies which sample value to use (e.g., 0 for samples, 1 for time/bytes).
func BuildFlameGraphTree(p *profile.Profile, valueIndex int) (*FlameGraphNode, error) {
	if valueIndex < 0 || valueIndex >= len(p.SampleType) {
		return nil, fmt.Errorf("invalid value index %d for profile with %d sample types", valueIndex, len(p.SampleType))
	}

	objectsIndex := -1
	isMemoryProfile := false
	valueUnit := p.SampleType[valueIndex].Unit
	valueType := p.SampleType[valueIndex].Type

	// Check if this is a memory profile (heap or allocs)
	// Memory profiles have bytes as the unit and specific value types
	if valueUnit == "bytes" && (valueType == "inuse_space" || valueType == "alloc_space" ||
		valueType == "alloc" || valueType == "allocation") {
		isMemoryProfile = true
		// Find the corresponding objects index
		for i, st := range p.SampleType {
			if (st.Type == "inuse_objects" || st.Type == "alloc_objects") && st.Unit == "count" {
				objectsIndex = i
				break
			}
		}
	}

	// Use a map to aggregate values for each unique call stack node (function)
	root := &tempNode{
		// Root node for the flame graph, represents the total profile value.
		// Name "root" is conventional for d3-flame-graph.
		node:        &FlameGraphNode{Name: "root", Value: 0},
		children:    make(map[nodeKey]*tempNode),
		selfValue:   0, // Root itself doesn't have a self value in this model
		objectCount: 0,
	}

	totalSampleValue := int64(0)
	totalObjectCount := int64(0)

	for _, sample := range p.Sample {
		value := sample.Value[valueIndex]
		if value == 0 {
			continue // Skip samples with zero value for the selected index
		}
		totalSampleValue += value

		var objCount int64 = 0
		if isMemoryProfile && objectsIndex >= 0 && len(sample.Value) > objectsIndex {
			objCount = sample.Value[objectsIndex]
			totalObjectCount += objCount
		}

		typeName := ""
		if isMemoryProfile && len(sample.Label) > 0 {
			if typeLabels, ok := sample.Label["type"]; ok && len(typeLabels) > 0 {
				typeName = typeLabels[0]
			} else if objLabels, ok := sample.Label["object"]; ok && len(objLabels) > 0 {
				typeName = objLabels[0]
			}
		}

		// Process the stack trace in reverse order (caller to callee for flame graph)
		currentNode := root
		for i := len(sample.Location) - 1; i >= 0; i-- {
			loc := sample.Location[i]
			// Aggregate by function for simplicity first.
			// A location can have multiple lines (e.g., due to inlining). We take the first line's function.
			if len(loc.Line) == 0 {
				continue // Skip locations without line info
			}
			line := loc.Line[0]
			fn := line.Function
			if fn == nil {
				// Use a placeholder name if function is unknown
				// Alternatively, could use loc.Address or other identifiers
				fn = &profile.Function{ID: 0, Name: fmt.Sprintf("unknown @ 0x%x", loc.Address)}
				// continue // Or skip lines without function info? Let's use a placeholder.
			}

			key := nodeKey{funcID: fn.ID}
			childNode, exists := currentNode.children[key]
			if !exists {
				childNode = &tempNode{
					node: &FlameGraphNode{
						Name:     fn.Name, // Use function name
						Value:    0,       // Will be calculated later
						Children: []*FlameGraphNode{},
						FilePath: fn.Filename,
						LineNum:  int(line.Line),
					},
					children:    make(map[nodeKey]*tempNode),
					selfValue:   0,
					objectCount: 0,
					filePath:    fn.Filename,
					lineNum:     int(line.Line),
					objectType:  typeName,
				}
				currentNode.children[key] = childNode
			}

			// Add the value to the selfValue of the *leaf* node in this sample's stack trace
			// This represents the time/memory spent directly in this function for this sample.
			if i == 0 {
				childNode.selfValue += value
				if isMemoryProfile && objCount > 0 {
					childNode.objectCount += objCount
					if typeName != "" && childNode.objectType == "" {
						childNode.objectType = typeName
					}
				}
			}

			// Move to the next level in the tree for the next location in the stack
			currentNode = childNode
		}
	}

	// Now, recursively calculate the total value (self + children) for each node
	// and build the final tree structure.
	calculateTotalValueAndBuildTree(root, isMemoryProfile, valueUnit)

	// Set the root's value to the total sample value calculated during the first pass.
	// calculateTotalValueAndBuildTree should also yield the same result if root.selfValue is 0.
	root.node.Value = totalSampleValue
	if isMemoryProfile {
		root.node.ValueFormatted = FormatBytes(totalSampleValue)
		if totalObjectCount > 0 {
			root.node.ObjectCount = totalObjectCount
			avgSize := int64(0)
			if totalObjectCount > 0 {
				avgSize = totalSampleValue / totalObjectCount
			}
			root.node.AvgSize = avgSize
			root.node.AvgSizeFormatted = FormatBytes(avgSize)
		}
	} else if valueUnit == "nanoseconds" {
		root.node.ValueFormatted = FormatSampleValue(totalSampleValue, valueUnit)
	}

	// Optional: Sort children nodes by value (descending) for potentially better visualization ordering.
	sortChildrenByValue(root.node)

	return root.node, nil
}

// calculateTotalValueAndBuildTree recursively calculates the total value (self + children)
// for each node and constructs the final FlameGraphNode children slice.
func calculateTotalValueAndBuildTree(tn *tempNode, isMemoryProfile bool, valueUnit string) int64 {
	// Start with the value directly attributed to this node
	total := tn.selfValue
	totalObjects := tn.objectCount
	childrenNodes := []*FlameGraphNode{} // Build the final children list here

	for _, childTempNode := range tn.children {
		// Recursively calculate the total value for the child
		childTotal := calculateTotalValueAndBuildTree(childTempNode, isMemoryProfile, valueUnit)
		// Set the final calculated value on the child's FlameGraphNode
		childTempNode.node.Value = childTotal
		childTempNode.node.SelfValue = childTempNode.selfValue

		if isMemoryProfile {
			childTempNode.node.ValueFormatted = FormatBytes(childTotal)
			childTempNode.node.ObjectCount = childTempNode.objectCount
			totalObjects += childTempNode.objectCount

			if childTempNode.objectCount > 0 {
				avgSize := childTotal / childTempNode.objectCount
				childTempNode.node.AvgSize = avgSize
				childTempNode.node.AvgSizeFormatted = FormatBytes(avgSize)
			}

			if childTempNode.objectType != "" {
				childTempNode.node.Type = childTempNode.objectType
			}
		} else if valueUnit == "nanoseconds" {
			childTempNode.node.ValueFormatted = FormatSampleValue(childTotal, valueUnit)
		}

		if childTempNode.filePath != "" {
			childTempNode.node.FilePath = childTempNode.filePath
		}
		if childTempNode.lineNum > 0 {
			childTempNode.node.LineNum = childTempNode.lineNum
		}

		// Only include children that ended up with a non-zero total value
		if childTotal > 0 {
			childrenNodes = append(childrenNodes, childTempNode.node)
		}
		// Add the child's total value to the current node's total
		total += childTotal
	}
	// Assign the final list of children to the current node's FlameGraphNode
	tn.node.Children = childrenNodes
	tn.node.SelfValue = tn.selfValue

	// Return the calculated total value for this node
	return total
}

// sortChildrenByValue recursively sorts the children of a FlameGraphNode by value (descending).
func sortChildrenByValue(node *FlameGraphNode) {
	if node == nil || len(node.Children) == 0 {
		return
	}
	// Sort the immediate children
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Value > node.Children[j].Value // Descending order
	})
	// Recursively sort the children of each child
	for _, child := range node.Children {
		sortChildrenByValue(child)
	}
}
