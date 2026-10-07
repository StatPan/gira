package gira

import (
	"fmt"
	"strings"
	"testing"
)

type goalStatusQuerySelection struct {
	Name          string
	Alias         string
	TypeCondition string
	Children      []goalStatusQuerySelection
}

type goalStatusQueryParser struct {
	tokens []string
	index  int
}

func TestGoalStatusIssueSnapshotQueryGrammarAndNesting(t *testing.T) {
	childSets := [][]int{
		{655},
		{703, 651, 693, 652, 700, 656, 699, 657, 683, 658, 688, 659, 696, 673, 687, 677, 689, 680},
	}
	wantNumbers := [][]int{
		{655},
		{651, 652, 656, 657, 658, 659, 673, 677, 680, 683, 687, 688, 689, 693, 696, 699, 700, 703},
	}
	for i, children := range childSets {
		query := goalStatusIssueSnapshotQuery(children)
		operation, err := parseGoalStatusQuery(query)
		if err != nil {
			t.Fatalf("query with %d child aliases is invalid: %v", len(children), err)
		}
		assertGoalStatusQueryShape(t, operation, wantNumbers[i])
	}
	if _, _, _, _, _, _, err := goalStatusIssueSnapshot(RepoRef{Owner: "StatPan", Name: "gira"}, make([]int, goalStatusRepositoryChildLimit+1), &goalStatusCountingRunner{}); err == nil {
		t.Fatal("snapshot builder should preserve the repository child limit")
	}
}

func parseGoalStatusQuery(query string) (goalStatusQuerySelection, error) {
	tokens, err := lexGoalStatusQuery(query)
	if err != nil {
		return goalStatusQuerySelection{}, err
	}
	p := goalStatusQueryParser{tokens: tokens}
	if !p.take("query") {
		return goalStatusQuerySelection{}, fmt.Errorf("expected query operation")
	}
	if p.peek() == "(" {
		if err := p.skipBalanced("(", ")"); err != nil {
			return goalStatusQuerySelection{}, err
		}
	}
	operation, err := p.selectionSet()
	if err != nil {
		return goalStatusQuerySelection{}, err
	}
	if p.index != len(p.tokens) {
		return goalStatusQuerySelection{}, fmt.Errorf("unexpected token %q after operation", p.peek())
	}
	return operation, nil
}

func lexGoalStatusQuery(query string) ([]string, error) {
	tokens := []string{}
	for i := 0; i < len(query); {
		c := query[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ',':
			i++
		case c == '#':
			for i < len(query) && query[i] != '\n' {
				i++
			}
		case c == '"':
			start := i
			i++
			for i < len(query) {
				if query[i] == '\\' {
					i += 2
					continue
				}
				if query[i] == '"' {
					i++
					break
				}
				i++
			}
			if i > len(query) || query[i-1] != '"' {
				return nil, fmt.Errorf("unterminated string")
			}
			tokens = append(tokens, query[start:i])
		case strings.HasPrefix(query[i:], "..."):
			tokens = append(tokens, "...")
			i += 3
		case isGoalStatusGraphQLNameStart(c):
			start := i
			i++
			for i < len(query) && isGoalStatusGraphQLNameContinue(query[i]) {
				i++
			}
			tokens = append(tokens, query[start:i])
		case strings.ContainsRune("!$():=@[]{|}&", rune(c)):
			tokens = append(tokens, string(c))
			i++
		case c == '-' || (c >= '0' && c <= '9'):
			start := i
			i++
			for i < len(query) && (isGoalStatusGraphQLNameContinue(query[i]) || query[i] == '.' || query[i] == '-') {
				i++
			}
			tokens = append(tokens, query[start:i])
		default:
			return nil, fmt.Errorf("unexpected character %q", c)
		}
	}
	return tokens, nil
}

func isGoalStatusGraphQLNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isGoalStatusGraphQLNameContinue(c byte) bool {
	return isGoalStatusGraphQLNameStart(c) || (c >= '0' && c <= '9')
}

func (p *goalStatusQueryParser) selectionSet() (goalStatusQuerySelection, error) {
	if !p.take("{") {
		return goalStatusQuerySelection{}, fmt.Errorf("expected selection set")
	}
	selection := goalStatusQuerySelection{}
	for p.index < len(p.tokens) && p.peek() != "}" {
		child, err := p.selection()
		if err != nil {
			return goalStatusQuerySelection{}, err
		}
		selection.Children = append(selection.Children, child)
	}
	if !p.take("}") {
		return goalStatusQuerySelection{}, fmt.Errorf("unclosed selection set")
	}
	if len(selection.Children) == 0 {
		return goalStatusQuerySelection{}, fmt.Errorf("empty selection set")
	}
	return selection, nil
}

func (p *goalStatusQueryParser) selection() (goalStatusQuerySelection, error) {
	if p.take("...") {
		if !p.take("on") {
			return goalStatusQuerySelection{}, fmt.Errorf("expected inline fragment type condition")
		}
		typeName := p.nextName()
		if typeName == "" {
			return goalStatusQuerySelection{}, fmt.Errorf("missing inline fragment type")
		}
		children, err := p.selectionSet()
		if err != nil {
			return goalStatusQuerySelection{}, err
		}
		return goalStatusQuerySelection{TypeCondition: typeName, Children: children.Children}, nil
	}
	first := p.nextName()
	if first == "" {
		return goalStatusQuerySelection{}, fmt.Errorf("expected field name, got %q", p.peek())
	}
	selection := goalStatusQuerySelection{Name: first}
	if p.take(":") {
		selection.Alias = first
		selection.Name = p.nextName()
		if selection.Name == "" {
			return goalStatusQuerySelection{}, fmt.Errorf("missing aliased field name")
		}
	}
	if p.peek() == "(" {
		if err := p.skipBalanced("(", ")"); err != nil {
			return goalStatusQuerySelection{}, err
		}
	}
	for p.take("@") {
		if p.nextName() == "" {
			return goalStatusQuerySelection{}, fmt.Errorf("missing directive name")
		}
		if p.peek() == "(" {
			if err := p.skipBalanced("(", ")"); err != nil {
				return goalStatusQuerySelection{}, err
			}
		}
	}
	if p.peek() == "{" {
		children, err := p.selectionSet()
		if err != nil {
			return goalStatusQuerySelection{}, err
		}
		selection.Children = children.Children
	}
	return selection, nil
}

func (p *goalStatusQueryParser) skipBalanced(open, close string) error {
	if !p.take(open) {
		return fmt.Errorf("expected %q", open)
	}
	stack := []string{close}
	for p.index < len(p.tokens) && len(stack) > 0 {
		token := p.tokens[p.index]
		p.index++
		switch token {
		case "(":
			stack = append(stack, ")")
		case "[":
			stack = append(stack, "]")
		case "{":
			stack = append(stack, "}")
		case ")", "]", "}":
			if stack[len(stack)-1] != token {
				return fmt.Errorf("mismatched delimiter %q", token)
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 0 {
		return fmt.Errorf("unclosed group %q", open)
	}
	return nil
}

func (p *goalStatusQueryParser) peek() string {
	if p.index >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.index]
}

func (p *goalStatusQueryParser) take(want string) bool {
	if p.peek() != want {
		return false
	}
	p.index++
	return true
}

func (p *goalStatusQueryParser) nextName() string {
	value := p.peek()
	if value == "" || !isGoalStatusGraphQLNameStart(value[0]) {
		return ""
	}
	p.index++
	return value
}

func assertGoalStatusQueryShape(t *testing.T, operation goalStatusQuerySelection, childNumbers []int) {
	t.Helper()
	if len(operation.Children) != 1 || operation.Children[0].Name != "repository" {
		t.Fatalf("root selection = %+v, want repository", operation.Children)
	}
	repository := operation.Children[0]
	if len(repository.Children) != len(childNumbers) {
		t.Fatalf("repository aliases = %d, want %d", len(repository.Children), len(childNumbers))
	}
	for i, issue := range repository.Children {
		wantAlias := fmt.Sprintf("issue%d", childNumbers[i])
		if issue.Name != "issue" || issue.Alias != wantAlias {
			t.Fatalf("issue selection %d = %+v, want alias %s", i, issue, wantAlias)
		}
		timeline, ok := goalStatusQueryChild(issue, "timelineItems")
		if !ok {
			t.Fatalf("%s has no timelineItems connection", wantAlias)
		}
		if _, ok := goalStatusQueryChild(timeline, "pageInfo"); !ok {
			t.Fatalf("%s timelineItems.pageInfo is not selected directly", wantAlias)
		}
		nodes, ok := goalStatusQueryChild(timeline, "nodes")
		if !ok {
			t.Fatalf("%s timelineItems.nodes is missing", wantAlias)
		}
		if _, nestedPageInfo := goalStatusQueryChild(nodes, "pageInfo"); nestedPageInfo {
			t.Fatalf("%s timelineItems.pageInfo is incorrectly nested under nodes", wantAlias)
		}
		crossRef := goalStatusQueryTypeChild(nodes, "CrossReferencedEvent")
		if crossRef == nil {
			t.Fatalf("%s lost the CrossReferencedEvent fragment", wantAlias)
		}
		source, ok := goalStatusQueryChild(*crossRef, "source")
		if !ok {
			t.Fatalf("%s lost cross-referenced source", wantAlias)
		}
		pr := goalStatusQueryTypeChild(source, "PullRequest")
		if pr == nil {
			t.Fatalf("%s lost the PullRequest fragment", wantAlias)
		}
		for _, field := range []string{"number", "title", "body", "state", "url", "isDraft", "mergeStateStatus", "reviewDecision", "headRefName", "baseRefName", "headRefOid", "baseRefOid", "mergeCommit", "reviews", "statusCheckRollup"} {
			if _, ok := goalStatusQueryChild(*pr, field); !ok {
				t.Fatalf("%s PullRequest selection lost %s", wantAlias, field)
			}
		}
		reviews, ok := goalStatusQueryChild(*pr, "reviews")
		if !ok || !goalStatusQueryHasChildren(reviews, "nodes", "pageInfo") {
			t.Fatalf("%s reviews connection is incomplete", wantAlias)
		}
		rollup, ok := goalStatusQueryChild(*pr, "statusCheckRollup")
		if !ok {
			t.Fatalf("%s status check rollup is missing", wantAlias)
		}
		contexts, ok := goalStatusQueryChild(rollup, "contexts")
		if !ok || !goalStatusQueryHasChildren(contexts, "nodes", "pageInfo") {
			t.Fatalf("%s status contexts connection is incomplete", wantAlias)
		}
	}
}

func goalStatusQueryChild(parent goalStatusQuerySelection, name string) (goalStatusQuerySelection, bool) {
	for _, child := range parent.Children {
		if child.Name == name {
			return child, true
		}
	}
	return goalStatusQuerySelection{}, false
}

func goalStatusQueryTypeChild(parent goalStatusQuerySelection, typeName string) *goalStatusQuerySelection {
	for i := range parent.Children {
		if parent.Children[i].TypeCondition == typeName {
			return &parent.Children[i]
		}
	}
	return nil
}

func goalStatusQueryHasChildren(parent goalStatusQuerySelection, names ...string) bool {
	for _, name := range names {
		if _, ok := goalStatusQueryChild(parent, name); !ok {
			return false
		}
	}
	return true
}
