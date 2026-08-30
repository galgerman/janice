// Package jsondocument contains the logic for rendering a Fyne tree from a JSON document.
package jsondocument

import (
	"context"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

const (
	// Update progress after x added nodes
	progressUpdateTick = 10_000
	// Total number of load steps
	totalLoadSteps = 3
	// Parent ID of root node
	rootNodeParentID = -1
	// Search target not found
	notFound = -1
)

var ErrCallerCanceled = errors.New("process canceled by caller")
var ErrNotFound = errors.New("not found")

// JSONType represents the type of a JSON value.
type JSONType uint8

const (
	Undefined JSONType = iota
	Array
	Boolean
	Null
	Number
	Object
	String
	Unknown
)

var typeMap = map[JSONType]string{
	Array:     "array",
	Boolean:   "boolean",
	Null:      "null",
	Number:    "number",
	Object:    "object",
	String:    "string",
	Undefined: "undefined",
	Unknown:   "unknown",
}

func (t JSONType) String() string {
	s, ok := typeMap[t]
	if !ok {
		return typeMap[Undefined]
	}
	return s
}

// SearchType represents the type of search to perform.
type SearchType uint

const (
	SearchKey SearchType = iota
	SearchString
	SearchNumber
	SearchKeyword
)

// SearchDirection controls traversal through search matches.
type SearchDirection int

const (
	SearchForward  SearchDirection = 1
	SearchBackward SearchDirection = -1
)

// ProgressInfo represents the current progress while loading a document
// and is used to communicate the the UI.
type ProgressInfo struct {
	CurrentStep int
	Progress    float64
	Size        int
	TotalSteps  int
}

// This singleton represents an empty value in a Node.
var Empty = struct{}{}

// Node represents a node in the JSON data tree.
type Node struct {
	Key   string
	Value any
	Type  JSONType
}

// JSONDocument represents a JSON document which can be rendered by a Fyne tree widget.
type JSONDocument struct {
	// How often progress info is updated
	ProgressUpdateTick int32

	progressInfo  binding.Untyped
	elementsCount int
	isJSONLines   bool
	previewKeys   []string

	// ids are stored as int32 to save memory. The API converts them to and from UID strings.
	ids     map[int32][]int32
	values  []Node  // using a slice here instead of a map for better load time
	parents []int32 // ditto
	n       int32
}

// Returns a new JSONDocument object.
func New() *JSONDocument {
	j := &JSONDocument{
		progressInfo:       binding.NewUntyped(),
		ProgressUpdateTick: progressUpdateTick,
	}
	j.initialize(0)
	return j
}

// ChildUIDs returns the child UIDs for a given node.
// This can be used directly in the tree widget childUIDs() function.
func (j *JSONDocument) ChildUIDs(uid widget.TreeNodeID) []widget.TreeNodeID {
	id := uid2id(uid)
	return ids2uids(j.ids[id])
}

// IsBranch reports whether a node is a branch.
// This can be used directly in the tree widget isBranch() function.
func (j *JSONDocument) IsBranch(uid widget.TreeNodeID) bool {
	id := uid2id(uid)
	_, found := j.ids[id]
	return found
}

// Value returns the value of a node
func (j *JSONDocument) Value(uid widget.TreeNodeID) Node {
	id := uid2id(uid)
	return j.values[id]
}

// SetKey renames an object property. Array indexes and the document root cannot
// be renamed.
func (j *JSONDocument) SetKey(uid widget.TreeNodeID, key string) error {
	id := uid2id(uid)
	key = strings.TrimSpace(key)
	if id <= 0 || key == "" {
		return fmt.Errorf("key can not be empty")
	}
	parentID := j.parents[id]
	if parentID < 0 || j.values[parentID].Type != Object {
		return fmt.Errorf("only object keys can be renamed")
	}
	for _, siblingID := range j.ids[parentID] {
		if siblingID != id && j.values[siblingID].Key == key {
			return fmt.Errorf("key %q already exists", key)
		}
	}
	j.values[id].Key = key
	return nil
}

// CanSetKey reports whether a node is an object property that can be renamed.
func (j *JSONDocument) CanSetKey(uid widget.TreeNodeID) bool {
	id := uid2id(uid)
	if id <= 0 || int(id) >= len(j.parents) {
		return false
	}
	parentID := j.parents[id]
	return parentID >= 0 && j.values[parentID].Type == Object
}

// SetScalarValue updates a scalar node from its textual representation while
// preserving the node's JSON type.
func (j *JSONDocument) SetScalarValue(uid widget.TreeNodeID, value string) error {
	id := uid2id(uid)
	if id < 0 || int(id) >= len(j.values) {
		return fmt.Errorf("invalid node")
	}
	n := &j.values[id]
	switch n.Type {
	case String:
		n.Value = value
	case Number:
		x, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return fmt.Errorf("invalid number: %w", err)
		}
		n.Value = x
	case Boolean:
		x, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("boolean must be true or false")
		}
		n.Value = x
	case Null:
		if strings.TrimSpace(value) != "null" {
			return fmt.Errorf("null value must remain null")
		}
		n.Value = nil
	default:
		return fmt.Errorf("arrays and objects can not be edited as values")
	}
	return nil
}

// ScalarText returns the unquoted text representation of a scalar node.
func (j *JSONDocument) ScalarText(uid widget.TreeNodeID) (string, bool) {
	n := j.Value(uid)
	switch n.Type {
	case String:
		return n.Value.(string), true
	case Number:
		return strconv.FormatFloat(n.Value.(float64), 'f', -1, 64), true
	case Boolean:
		return strconv.FormatBool(n.Value.(bool)), true
	case Null:
		return "null", true
	default:
		return "", false
	}
}

// Marshal serializes the complete document. JSON Lines documents are emitted
// as one compact JSON value per line.
func (j *JSONDocument) Marshal() ([]byte, error) {
	if j.n == 0 {
		return nil, fmt.Errorf("document is empty")
	}
	if j.isJSONLines {
		var result strings.Builder
		for _, rowID := range j.ids[0] {
			data, err := json.Marshal(j.extractValue(rowID))
			if err != nil {
				return nil, err
			}
			result.Write(data)
			result.WriteByte('\n')
		}
		return []byte(result.String()), nil
	}
	return json.MarshalIndent(j.extractValue(0), "", "  ")
}

func (j *JSONDocument) extractValue(id int32) any {
	n := j.values[id]
	switch n.Type {
	case Array:
		return j.extractArray(id)
	case Object:
		return j.extractObject(id)
	default:
		return n.Value
	}
}

// Load loads JSON data from a reader and builds a new JSON document from it.
// It reports it's current progress to the caller via updates to progressInfo.
// Closes the reader.
func (j *JSONDocument) Load(ctx context.Context, reader fyne.URIReadCloser, progressInfo binding.Untyped) error {
	j.progressInfo = progressInfo
	isJSONLinesFile := strings.EqualFold(filepath.Ext(reader.URI().Path()), ".jsonl")
	data, isJSONLines, err := j.loadWithFormat(ctx, reader)
	if errors.Is(err, context.Canceled) {
		err = ErrCallerCanceled
	}
	if err != nil {
		return err
	}
	if isJSONLinesFile && !isJSONLines {
		data = []any{data}
		isJSONLines = true
	}
	select {
	case <-ctx.Done():
		return ErrCallerCanceled
	default:
	}
	if err := j.setProgressInfo(ProgressInfo{CurrentStep: 2}); err != nil {
		return err
	}
	sizer := JSONTreeSizer{}
	size, err := sizer.Calculate(data)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ErrCallerCanceled
	default:
	}
	j.elementsCount = size
	slog.Info("Tree size calculated", "size", size)
	if err := j.setProgressInfo(ProgressInfo{CurrentStep: 3}); err != nil {
		return err
	}
	if err := j.render(ctx, data, int32(size)); err != nil {
		return err
	}
	j.isJSONLines = isJSONLines
	if isJSONLines {
		j.previewKeys = collectPreviewKeys(data)
	}
	data = nil // GC can free this memory
	slog.Info("Finished loading JSON document into tree", "size", j.n)
	return nil
}

// Size returns the number of nodes.
func (j *JSONDocument) Reset() {
	j.initialize(0)
	j.isJSONLines = false
	j.previewKeys = nil
}

// IsJSONLines reports whether the document was loaded from a stream containing
// multiple JSON values (JSON Lines).
func (j *JSONDocument) IsJSONLines() bool {
	return j.isJSONLines
}

// JSONLinesRowCount returns the number of top-level JSON Lines records.
func (j *JSONDocument) JSONLinesRowCount() int {
	if !j.isJSONLines {
		return 0
	}
	return len(j.ids[0])
}

// JSONLinesPreviewKeys returns the scalar keys available on top-level JSON
// Lines objects. The returned slice is safe for callers to modify.
func (j *JSONDocument) JSONLinesPreviewKeys() []string {
	return slices.Clone(j.previewKeys)
}

// JSONLinesRowUID returns the tree UID for a zero-based JSON Lines row.
func (j *JSONDocument) JSONLinesRowUID(row int) (widget.TreeNodeID, bool) {
	if !j.isJSONLines || row < 0 || row >= len(j.ids[0]) {
		return "", false
	}
	return id2uid(j.ids[0][row]), true
}

// JSONLinesRowIndex returns the zero-based JSON Lines row containing uid.
func (j *JSONDocument) JSONLinesRowIndex(uid widget.TreeNodeID) int {
	if !j.isJSONLines || uid == "" {
		return -1
	}
	id := uid2id(uid)
	for j.parents[id] != 0 {
		id = j.parents[id]
		if id == rootNodeParentID {
			return -1
		}
	}
	return slices.Index(j.ids[0], id)
}

// JSONLinesRowPreview returns a formatted scalar value for key on the row
// represented by uid.
func (j *JSONDocument) JSONLinesRowPreview(uid widget.TreeNodeID, key string) (string, bool) {
	if uid == "" || j.parents[uid2id(uid)] != 0 {
		return "", false
	}
	row := j.JSONLinesRowIndex(uid)
	path := previewPathSegments(key)
	if row < 0 || len(path) == 0 {
		return "", false
	}
	id := j.ids[0][row]
	for _, segment := range path {
		found := false
		for _, childID := range j.ids[id] {
			if j.values[childID].Key == segment {
				id = childID
				found = true
				break
			}
		}
		if !found {
			return "", false
		}
	}
	n := j.values[id]
	switch n.Type {
	case String:
		return n.Value.(string), true
	case Number:
		return strconv.FormatFloat(n.Value.(float64), 'f', -1, 64), true
	case Boolean:
		return strconv.FormatBool(n.Value.(bool)), true
	case Null:
		return "null", true
	case Object:
		return "{...}", true
	case Array:
		return "[...]", true
	default:
		return "", false
	}
}

// PreviewPath returns a JSON Pointer-like path for a tree key. Paths in JSON
// Lines documents are relative to the containing row so they can be reused.
func (j *JSONDocument) PreviewPath(uid widget.TreeNodeID) (string, bool) {
	id := uid2id(uid)
	if id <= 0 || int(id) >= len(j.values) {
		return "", false
	}
	segments := make([]string, 0)
	for id > 0 {
		if j.isJSONLines && j.parents[id] == 0 {
			break
		}
		key := j.values[id].Key
		if key != "" {
			segments = append(segments, strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1"))
		}
		id = j.parents[id]
	}
	if len(segments) == 0 {
		return "", false
	}
	slices.Reverse(segments)
	return "/" + strings.Join(segments, "/"), true
}

func previewPathSegments(path string) []string {
	if path == "" {
		return nil
	}
	if !strings.HasPrefix(path, "/") {
		return []string{path}
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts
}

// Parent returns the UID of the parent node.
func (j *JSONDocument) Parent(uid widget.TreeNodeID) widget.TreeNodeID {
	id := uid2id(uid)
	return id2uid(j.parents[id])
}

// Path returns the path of a node in the tree.
func (j *JSONDocument) Path(uid widget.TreeNodeID) []widget.TreeNodeID {
	path := make([]int32, 0)
	id := uid2id(uid)
	for {
		id = j.parents[id]
		if id == 0 {
			break
		}
		path = append(path, id)
	}
	slices.Reverse(path)
	return ids2uids(path)
}

// Size returns the number of nodes.
func (j *JSONDocument) Size() int {
	return int(j.n)
}

// readCloserCtx adds context to a ReadCloser and allows a stream to be canceled.
type readCloserCtx struct {
	ctx context.Context
	r   io.ReadCloser
}

func (r *readCloserCtx) Read(p []byte) (n int, err error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func (r *readCloserCtx) Close() error {
	return r.r.Close()
}

// newReaderContext returns a new readCloser with a context.
func newReaderContext(ctx context.Context, r io.ReadCloser) io.ReadCloser {
	return &readCloserCtx{ctx: ctx, r: r}
}

func (j *JSONDocument) load(ctx context.Context, reader io.ReadCloser) (any, error) {
	data, _, err := j.loadWithFormat(ctx, reader)
	return data, err
}

func (j *JSONDocument) loadWithFormat(ctx context.Context, reader io.ReadCloser) (any, bool, error) {
	defer reader.Close()
	if err := j.setProgressInfo(ProgressInfo{CurrentStep: 1}); err != nil {
		return nil, false, err
	}
	var data any
	reader2 := newReaderContext(ctx, reader)
	dec := stdjson.NewDecoder(reader2)
	if err := dec.Decode(&data); err != nil {
		return nil, false, err
	}
	rows := []any{data}
	for {
		var row any
		err := dec.Decode(&row)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, false, err
		}
		rows = append(rows, row)
	}
	if len(rows) == 1 {
		return data, false, nil
	}
	return rows, true, nil
}

func collectPreviewKeys(data any) []string {
	rows, ok := data.([]any)
	if !ok {
		return nil
	}
	keys := make(map[string]struct{})
	for _, row := range rows {
		object, ok := row.(map[string]any)
		if !ok {
			continue
		}
		for key, value := range object {
			switch value.(type) {
			case string, float64, bool, nil:
				keys[key] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	slices.Sort(result)
	return result
}

// render is the main method for rendering the JSON data into a tree.
func (j *JSONDocument) render(ctx context.Context, data any, size int32) error {
	j.initialize(size)
	var err error
	switch v := data.(type) {
	case map[string]any:
		if _, err := j.addNode(ctx, -1, "", Empty, Object); err != nil {
			return err
		}
		err = j.addObject(ctx, 0, v)
	case []any:
		if _, err := j.addNode(ctx, -1, "", Empty, Array); err != nil {
			return err
		}
		err = j.addArray(ctx, 0, v)
	default:
		err = fmt.Errorf("unrecognized format")
	}
	return err
}

// addObject adds a JSON object to the tree.
func (j *JSONDocument) addObject(ctx context.Context, parentID int32, data map[string]any) error {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		v := data[k]
		if err := j.addValue(ctx, parentID, k, v); err != nil {
			return err
		}
	}
	return nil
}

// addArray adds a JSON array to the tree.
func (j *JSONDocument) addArray(ctx context.Context, parentID int32, a []any) error {
	var sb strings.Builder
	for i, v := range a {
		sb.Reset()
		sb.WriteByte('[')
		sb.WriteString(strconv.Itoa(i))
		sb.WriteByte(']')
		if err := j.addValue(ctx, parentID, sb.String(), v); err != nil {
			return err
		}
	}
	return nil
}

// addValue adds a JSON value to the tree.
func (j *JSONDocument) addValue(ctx context.Context, parentID int32, k string, v any) error {
	switch v2 := v.(type) {
	case map[string]any:
		id, err := j.addNode(ctx, parentID, k, Empty, Object)
		if err != nil {
			return err
		}
		if err := j.addObject(ctx, id, v2); err != nil {
			return err
		}
	case []any:
		id, err := j.addNode(ctx, parentID, k, Empty, Array)
		if err != nil {
			return err
		}
		if err := j.addArray(ctx, id, v2); err != nil {
			return err
		}
	case string:
		_, err := j.addNode(ctx, parentID, k, v2, String)
		if err != nil {
			return err
		}
	case float64:
		_, err := j.addNode(ctx, parentID, k, v2, Number)
		if err != nil {
			return err
		}
	case bool:
		_, err := j.addNode(ctx, parentID, k, v2, Boolean)
		if err != nil {
			return err
		}
	case nil:
		_, err := j.addNode(ctx, parentID, k, v2, Null)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unrecognized JSON type %v", v)
	}
	return nil
}

// addNode adds a node to the tree and returns the UID.
// Nodes will be rendered in the same order they are added.
// parentID == -1 denotes the root node
// Returns the generated UID for this node and the incremented ID
func (j *JSONDocument) addNode(ctx context.Context, parentID int32, key string, value any, typ JSONType) (int32, error) {
	if parentID != rootNodeParentID {
		n := j.values[parentID]
		if n.Type == Undefined {
			return 0, fmt.Errorf("parent ID does not exist: %d", parentID)
		}
	}
	id := j.n
	n := j.values[id]
	if n.Type != Undefined {
		return 0, fmt.Errorf("ID for this node already exists: %v", id)
	}
	j.values[id] = Node{Key: key, Value: value, Type: typ}
	j.parents[id] = parentID
	if parentID != rootNodeParentID {
		j.ids[parentID] = append(j.ids[parentID], id)
	}
	if j.n%j.ProgressUpdateTick == 0 {
		select {
		case <-ctx.Done():
			return 0, ErrCallerCanceled
		default:
		}
		p := float64(j.n) / float64(j.elementsCount)
		if err := j.setProgressInfo(ProgressInfo{CurrentStep: 3, Progress: p}); err != nil {
			slog.Warn("Failed to set progress", "err", err)
		}
	}
	j.n++
	return id, nil
}

// initialize initializes the tree and allocates needed memory.
//
// A valid tree includes a root node (ID=0) and at least one normal node.
func (j *JSONDocument) initialize(size int32) {
	j.ids = make(map[int32][]int32)
	j.values = make([]Node, size)
	j.parents = make([]int32, size)
	j.n = 0
}

func (j *JSONDocument) setProgressInfo(info ProgressInfo) error {
	info.TotalSteps = totalLoadSteps
	info.Size = j.elementsCount
	if err := j.progressInfo.Set(info); err != nil {
		return err
	}
	return nil
}

// Search returns the next node with a matching key or value.
func (j *JSONDocument) Search(ctx context.Context, uid widget.TreeNodeID, search string, typ SearchType) (widget.TreeNodeID, error) {
	return j.SearchDirection(ctx, uid, search, typ, SearchForward)
}

// SearchDirection returns the next matching node in the requested direction.
// The starting node is ignored so successive calls navigate distinct matches.
func (j *JSONDocument) SearchDirection(ctx context.Context, uid widget.TreeNodeID, search string, typ SearchType, direction SearchDirection) (widget.TreeNodeID, error) {
	if search == "" {
		return "", ErrNotFound
	}
	pattern, err := regexp.Compile(wildCardToRegexp(search))
	if err != nil {
		return "", err
	}
	ids := j.flattenIDs(0)
	start := slices.Index(ids, uid2id(uid))
	if direction == SearchBackward {
		if uid == "" {
			start = len(ids)
		}
		for i := start - 1; i >= 0; i-- {
			if err := ctx.Err(); err != nil {
				return "", ErrCallerCanceled
			}
			if j.matches(ids[i], pattern, typ) {
				return id2uid(ids[i]), nil
			}
		}
		return "", ErrNotFound
	}
	for i := start + 1; i < len(ids); i++ {
		if err := ctx.Err(); err != nil {
			return "", ErrCallerCanceled
		}
		if j.matches(ids[i], pattern, typ) {
			return id2uid(ids[i]), nil
		}
	}
	return "", ErrNotFound
}

func (j *JSONDocument) flattenIDs(id int32) []int32 {
	result := []int32{id}
	for _, childID := range j.ids[id] {
		result = append(result, j.flattenIDs(childID)...)
	}
	return result
}

func (j *JSONDocument) matches(id int32, pattern *regexp.Regexp, typ SearchType) bool {
	n := j.values[id]
	switch typ {
	case SearchKey:
		return pattern.MatchString(n.Key)
	case SearchKeyword:
		if n.Type == Boolean {
			return pattern.MatchString(strconv.FormatBool(n.Value.(bool)))
		}
		return n.Type == Null && pattern.MatchString("null")
	case SearchNumber:
		return n.Type == Number && pattern.MatchString(strconv.FormatFloat(n.Value.(float64), 'f', -1, 64))
	case SearchString:
		return n.Type == String && pattern.MatchString(n.Value.(string))
	default:
		return false
	}
}

// Replace updates a matching key or scalar node with replacement text.
func (j *JSONDocument) Replace(uid widget.TreeNodeID, typ SearchType, replacement string) error {
	if typ == SearchKey {
		return j.SetKey(uid, replacement)
	}
	return j.SetScalarValue(uid, replacement)
}

// ReplaceAll replaces all matches and returns the number changed.
func (j *JSONDocument) ReplaceAll(ctx context.Context, search string, typ SearchType, replacement string) (int, error) {
	if search == "" {
		return 0, ErrNotFound
	}
	pattern, err := regexp.Compile(wildCardToRegexp(search))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, id := range j.flattenIDs(0) {
		if err := ctx.Err(); err != nil {
			return count, ErrCallerCanceled
		}
		if !j.matches(id, pattern, typ) {
			continue
		}
		if err := j.Replace(id2uid(id), typ, replacement); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (j *JSONDocument) searchNode(ctx context.Context, id int32, pattern *regexp.Regexp, typ SearchType) (int32, error) {
	n := j.values[id]
	if n.Type == Array || n.Type == Object {
		foundID, err := j.searchContainer(ctx, id, pattern, typ)
		if err != nil {
			return 0, err
		}
		if foundID != notFound {
			return foundID, nil
		}
	}
	switch typ {
	case SearchKey:
		if pattern.MatchString(n.Key) {
			return id, nil
		}
	case SearchKeyword:
		switch n.Type {
		case Boolean:
			if pattern.MatchString(fmt.Sprint(n.Value)) {
				return id, nil
			}
		case Null:
			if pattern.MatchString("null") {
				return id, nil
			}
		default:
			return notFound, nil
		}
	case SearchNumber:
		if n.Type != Number {
			return notFound, nil
		}
		v := n.Value.(float64)
		if pattern.MatchString(strconv.FormatFloat(v, 'f', -1, 64)) {
			return id, nil
		}
	case SearchString:
		if n.Type != String {
			return notFound, nil
		}
		v := n.Value.(string)
		if pattern.MatchString(v) {
			return id, nil
		}
	default:
		panic("Undefined search type")
	}
	return notFound, nil
}

func (j *JSONDocument) searchContainer(ctx context.Context, id int32, pattern *regexp.Regexp, typ SearchType) (int32, error) {
	select {
	case <-ctx.Done():
		return 0, ErrCallerCanceled
	default:
	}
	for _, childID := range j.ids[id] {
		foundID, err := j.searchNode(ctx, childID, pattern, typ)
		if err != nil {
			return 0, err
		}
		if foundID != notFound {
			return foundID, nil
		}
	}
	return notFound, nil
}

func wildCardToRegexp(pattern string) string {
	components := strings.Split(pattern, "*")
	if len(components) == 1 {
		// if len is 1, there are no *'s, return exact match pattern
		return "^" + pattern + "$"
	}
	var result strings.Builder
	for i, literal := range components {

		// Replace * with .*
		if i > 0 {
			result.WriteString(".*")
		}

		// Quote any regular expression meta characters in the
		// literal text.
		result.WriteString(regexp.QuoteMeta(literal))
	}
	return "^" + result.String() + "$"
}

// Extract returns a segment of the JSON document, with the given UID as new root container.
// Note that only arrays and objects can be extracted
func (j *JSONDocument) Extract(uid widget.TreeNodeID) ([]byte, error) {
	var data any
	id := uid2id(uid)
	n := j.values[id]
	switch n.Type {
	case Array:
		data = j.extractArray(id)
	case Object:
		data = j.extractObject(id)
	default:
		return nil, fmt.Errorf("can only extract objects and arrays")
	}
	return json.Marshal(data)
}

func (j *JSONDocument) extractArray(id int32) []any {
	data := make([]any, len(j.ids[id]))
	for i, childID := range j.ids[id] {
		data[i] = j.extractValue(childID)
	}
	return data
}

func (j *JSONDocument) extractObject(id int32) map[string]any {
	data := make(map[string]any)
	for _, childID := range j.ids[id] {
		n := j.values[childID]
		data[n.Key] = j.extractValue(childID)
	}
	return data
}

func uid2id(uid widget.TreeNodeID) int32 {
	if uid == "" {
		return 0
	}
	id, err := strconv.Atoi(uid)
	if err != nil {
		panic(err)
	}
	return int32(id)
}

func id2uid(id int32) widget.TreeNodeID {
	if id == 0 {
		return ""
	}
	return strconv.Itoa(int(id))
}

func ids2uids(ids []int32) []widget.TreeNodeID {
	uids := make([]widget.TreeNodeID, len(ids))
	for i, id := range ids {
		uids[i] = id2uid(id)
	}
	return uids
}
