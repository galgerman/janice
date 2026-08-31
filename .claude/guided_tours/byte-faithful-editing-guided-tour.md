# Byte-faithful editing in Janice — guided tour

Janice gained editing and Save in the `codex/jsonl-browser` branch, but Save
rebuilt the entire file from the in-memory tree. That normalised everything it
touched: key order, indentation, and number precision. This tour follows the
change that makes Save **byte-faithful** — the saved file differs from the
original *only* at the places the user actually edited.

## Legend

| Badge | Meaning |
|---|---|
| ✅ | Achieved / verified good |
| 🟡 | Special note — pay attention |
| 🔴 | Open issue / risk / unresolved |
| 🔵 | Context / rationale |
| 🟣 | Decision made (and why) |

Callouts: `> [!TIP]` good result · `> [!IMPORTANT]` must-know ·
`> [!WARNING]` open issue · `> [!NOTE]` context.

---

## 🔵 Exposition

Janice began as a viewer. The `codex/jsonl-browser` branch gave it editing and a
Save command, but Save worked the only way it could at the time: it walked the
in-memory tree and re-serialised the whole document. Everything the tree did not
model was therefore lost on every save — object key order, indentation, spacing,
line endings, and number precision (`12345678901234567890` came back as
`12345678901234567000`). Fixing one typo in a 500-line config rewrote all 500
lines, which in a diff buries the real change completely.

The goal of this change: **a saved file differs from the original only where the
user actually edited it.** Not "semantically equivalent" — byte-identical
everywhere else.

Getting there meant giving up whole-document re-serialisation entirely. The
document now keeps numbers as their source literals, records each edit as ready
made JSON text against a node id, and saves by *streaming the original file
through to the output*, substituting only the spans belonging to edited nodes.
Node ids are re-derived by walking the source in the same depth-first order the
loader used, so no byte offsets need to be stored during load — which matters,
because Janice is built for files far too large to keep a second copy of.

## Summary

| Phase | Outcome | Status |
|---|---|---|
| [1](#phase-1--numbers-keep-their-source-literal-) | Numbers stored as `json.Number` literals, so the tree stops rounding what the file said | ✅ |
| [2](#phase-2--the-document-remembers-what-was-edited-) | `nodeEdit` records each change as encoded JSON text, keyed by node id | ✅ |
| [3](#phase-3--saving-splices-instead-of-re-serialising-) | `WriteSpliced` streams the original through, substituting only edited spans | ✅ |
| [4](#phase-4--the-ui-saves-through-the-splicer-) | Save/Save As render to a temp file then rename; Save As renders *before* the dialog truncates | ✅ |
| [5](#phase-5--verification-at-scale-and-on-awkward-input-) | Byte-identity proven on 299,593 nodes and 12 awkward shapes | ✅ |

## Conclusion

Saving is now byte-faithful. Editing one value in the deliberately messy test
fixture — CRLF endings, ragged indentation, tabs, `é`, `1e3`, `1.10`,
`12345678901234567890` — changes exactly those four characters and nothing else,
and saving with no edits at all reproduces the file byte for byte, verified on a
4.2 MB / 299,593-node document.

A reviewer should start with **[splice.go](../../internal/jsondocument/splice.go)**,
specifically `splicer.value`: that one function contains both the id/type
correspondence the design rests on and the single point where a substitution
happens. Then read `saveFileAs` in [ui.go](../../internal/ui/ui.go) to see why the
content is rendered before the dialog opens — it looks inverted until you know
the dialog truncates.

Residual 🔴 issues are listed in Phase 5: top-level scalar documents still fail
to load (pre-existing), and structural edits — adding or removing nodes — remain
unsupported by design, since the splicer assumes the tree and the file have the
same shape.

_Anchored at `codex/jsonl-browser`, uncommitted working tree (2026-08-31)._

## Phase 1 — Numbers keep their source literal ✅

**Goal.** A JSON number was decoded into a Go `float64`, so `12345678901234567890`
became `12345678901234567000` and `1.10` became `1.1` — visibly, in the tree,
before any save happened. Byte-faithful saving is impossible while the value a
node holds is already a lossy approximation of the file.

🔵 **Core ideas introduced.**

*Numbers are stored as `encoding/json.Number` — the literal text, not a float.*
`json.Number` is a `string` underneath, holding exactly the characters that
appeared in the file (`1e3` stays `1e3`, not `1000`). It exists because IEEE-754
`float64` cannot represent every JSON number: integers beyond 2^53 are rounded,
and trailing zeros in a decimal are lost since they carry no numeric meaning.
Without it the tree cannot answer "what did the file actually say here", which
is the whole premise of the rest of this tour.

Consumers, by name: [`ScalarText`](../../internal/jsondocument/jsondocument.go) (the
edit dialog and the detail pane), [`matches`](../../internal/jsondocument/jsondocument.go)
(number search — it now matches against the literal, so searching `1.10` finds
`1.10`), [`JSONLinesRowPreview`](../../internal/jsondocument/jsondocument.go) (the row
preview column), and [tree.go](../../internal/ui/tree.go) which renders the value
beside each node.

Blast radius: any new code touching a `Number` node must type-assert
`stdjson.Number`, **not** `float64` — an assertion on the old type compiles but
panics at runtime. `SetScalarValue` no longer parses to a float either; it
validates the text is legal JSON number syntax via `isJSONNumber` and stores the
user's own characters, so an edited value is written exactly as typed.

**What changed.**
- `readOrdered` calls `iter.ReadNumber()` instead of `ReadFloat64()`, **so** the
  literal survives decoding intact.
- `addValue` accepts `stdjson.Number`, **so** the tree stores that literal.
- `ScalarText`, `matches` and `JSONLinesRowPreview` read the literal directly
  instead of reformatting a float, **so** display, search and preview all agree
  with the file.
- `SetScalarValue` validates syntax and keeps the typed text, **so** editing a
  number no longer silently renormalises it.

**Key code.**
- [ordered.go](../../internal/jsondocument/ordered.go) — `readOrdered`'s `NumberValue`
  case, and `isJSONNumber` which validates an edited literal by decoding it and
  requiring the whole string be consumed.
- [jsondocument.go](../../internal/jsondocument/jsondocument.go) — `ScalarText` and
  `SetScalarValue`; the `Number` cases are where the type assertion changed.

**Verification.**
```
go test ./internal/jsondocument/ -run TestNumbersKeepTheirOriginalLiteral -count=1
```
Failed first with `expected: "12345678901234567890" / actual: "12345678901234567000"`,
then after the change:
```
ok  	github.com/ErikKalkoken/janice/internal/jsondocument	0.748s
```
Full suite green: `ok internal/github`, `ok internal/jsondocument`, `ok internal/ui`.

**Notes & open issues.**
> [!NOTE]
> 🟡 This phase alone does not make Save faithful — it only stops the *tree* from
> lying about the file. Whole-document `Marshal` still reformats everything;
> Phases 2–4 are what stop Save from using it.

## Phase 2 — The document remembers *what* was edited ✅

**Goal.** Splicing needs to know which nodes changed and what text to put in
their place. The tree only knows its current state; it cannot tell an untouched
node from one the user retyped to the same value.

🔵 **Core ideas introduced.**

*`nodeEdit` — the replacement text for one changed node, stored already encoded.*
It holds `key`, `value` and the two `hasKey`/`hasValue` flags that distinguish
"changed to empty" from "not changed". Crucially it stores the **encoded JSON
text** (`"abc"`, `1.10`, `true`), not a Go value: the writer in Phase 3 then
needs no knowledge of JSON types at all — it copies bytes and, where an edit
exists, writes that string. Without this record, saving would have to re-encode
the whole tree, which is exactly the behaviour being removed.

The map lives on `JSONDocument.edits`, keyed by node id. Who reads it:
[splice.go](../../internal/jsondocument/splice.go) via `w.edits`, and `HasEdits()`
which the UI uses. It is written only by `recordEdit`, called from `SetKey` and
each successful branch of `SetScalarValue`.

Blast radius: **any future mutation must call `recordEdit`, or its change will
be silently dropped from the saved file.** `initialize` clears the map, so every
load and `Reset` starts clean — a stale edit applied to a freshly loaded
document would corrupt it.

**What changed.**
- Added the `edits` map and `recordEdit`, **so** each mutation leaves a record
  the writer can find by node id.
- `SetKey` and all four scalar branches of `SetScalarValue` record their
  replacement text, **so** what gets saved is what the user actually typed.
- `initialize` clears the map, **so** reloading cannot carry edits across
  documents.
- Added `encodeJSONString` with `SetEscapeHTML(false)`, **so** an edited string
  is escaped correctly without gratuitously mangling `<`, `>` and `&`.

**Key code.**
- [edits.go](../../internal/jsondocument/edits.go) — the whole mechanism; `recordEdit`
  is the single entry point.

**Verification.**
```
go test ./internal/jsondocument/ -run TestDocumentRecordsEdits -count=1
```
Failed first with `j.HasEdits undefined`, then:
```
ok  	github.com/ErikKalkoken/janice/internal/jsondocument	0.633s
```

**Notes & open issues.** None.

## Phase 3 — Saving splices instead of re-serialising ✅

**Goal.** Produce a saved file that differs from the original *only* where the
user edited, with no offset table stored during load (which would have cost
~8 bytes per node — 88 MB on an 11 M-node document).

🔵 **Core ideas introduced.**

*The splicer re-derives node ids by re-walking the source in the same order the
loader did.* Node ids are handed out depth-first as the tree is built, so a
second depth-first walk of the same bytes assigns the same id to the same value.
That correspondence is the entire trick: an edit recorded against id 57 can be
applied to the right bytes without ever having stored where node 57 lives.
`splicer.take()` is the counter that hands out those ids.

Without it, the alternatives were storing a byte span per node (memory) or
re-serialising the tree (what we are removing). The invariant it depends on is
the one pinned by `TestNodeIDsFollowDepthFirstOrder` — **if node ids ever stop
being assigned depth-first, this writer silently produces wrong output**, which
is why that test exists.

*The structural check.* Every value the walk enters is compared against
`doc.values[id].Type`, and at the end `w.next` must equal `doc.n`. A file edited
by another program since it was loaded therefore fails loudly instead of having
an edit spliced into the wrong place. This is the safety net for re-reading the
file from disk at save time rather than holding a second copy in memory.

🟣 **Decision: the source is re-read, not retained.** Janice exists to open very
large files; keeping a second full copy purely so Save can diff against it would
undo that. The writer streams through a `bufio.Reader`, so it needs only its
64 KB buffer regardless of file size — paid for with one extra pass at save
time, on an action a user takes rarely.

**What changed.**
- Added `WriteSpliced(src, dst)` which copies the source through byte for byte,
  **so** indentation, spacing, line endings, key order and number formatting are
  preserved by construction rather than by careful re-encoding.
- Value substitution consumes the original scalar without emitting it and writes
  the recorded text instead, **so** only the edited token changes.
- The JSON Lines branch skips the synthetic root array — id 0, which occupies no
  bytes — **so** row ids line up with a source that is a stream of bare values.
- Every entered value is type-checked against the tree, **so** a drifted source
  is refused rather than mis-spliced.

**Key code.**
- [splice.go](../../internal/jsondocument/splice.go) — `run` (the JSON Lines vs single
  value split), `value` (the type check and the substitution point), and
  `stringToken` which must track escapes to find the true end of a string.

**Verification.**
```
go test ./internal/jsondocument/ -run TestSplice -count=1 -v
```
All seven pass, including a deliberately awkward fixture with CRLF line endings,
tabs, ragged spacing, `é`, `1e3`, `1.10` and `12345678901234567890`:
```
--- PASS: TestSpliceWithoutEditsIsByteIdentical
--- PASS: TestSpliceReplacesOnlyTheEditedValue
--- PASS: TestSpliceReplacesOnlyTheEditedKey
--- PASS: TestSpliceEditsNestedArrayElement
--- PASS: TestSpliceEscapesEditedStrings
--- PASS: TestSpliceJSONLinesTouchesOnlyTheEditedRow
--- PASS: TestSpliceRejectsASourceThatNoLongerMatches
ok  	github.com/ErikKalkoken/janice/internal/jsondocument	0.690s
```

**Notes & open issues.**
> [!IMPORTANT]
> 🟡 `WriteSpliced` must be given the *same* bytes the document was loaded from.
> Phase 4 is what guarantees the UI actually does that.

## Phase 4 — The UI saves through the splicer ✅

**Goal.** Make Save and Save As actually use `WriteSpliced`, and do it without a
window in which a failed or interrupted save can destroy the user's file.

🔵 **Core ideas introduced.**

*Render first, then move into place.* `renderDocument` writes the finished
content to a **temporary file** and returns its path; the caller then renames
that file over the target. Nothing ever writes into the destination directly, so
a crash or a splice error mid-write leaves the original file untouched — and the
rename is atomic on both Windows and Unix.

*Why Save As renders **before** opening the dialog.* Fyne's save dialog produces
its writer with `os.Create`, which **truncates the chosen file the instant the
dialog returns** (see `openFile` in `fyne/internal/repository/file.go`). If the
user picks the file the document was loaded from, its bytes are gone before our
callback runs — and those bytes are exactly what the splicer needs. Rendering
first means the content is already materialised on disk before the dialog can
destroy anything. This is the subtlest constraint in the whole change, and the
reason `saveFileAs` looks back-to-front.

Consumers: `saveFile` and `saveFileAs` in [ui.go](../../internal/ui/ui.go) are the
only callers. `moveFile` falls back to a byte copy when rename fails across
volumes, which Save As can hit since its temp file lives in the system temp dir.

Blast radius: `afterSave` **must** call `ClearEdits`, because the saved file is
now the baseline for the next splice — a stale edit would be re-applied against
a file that already contains it.

🟣 **Decision: a splice failure is reported, never silently downgraded.** If the
file on disk drifted, falling back to re-serialising would quietly reformat the
user's entire document — the exact behaviour this change exists to remove. Only
a *missing* original (a document with no file behind it) falls back to
`Marshal`, and then the UI says so.

**What changed.**
- `saveFile` renders to a temp file beside the target and renames it in, **so**
  the original survives any failure.
- `saveFileAs` renders before showing the dialog, **so** choosing the source file
  as the destination cannot destroy the bytes being read.
- `writeDocumentTo` splices when an original exists and otherwise re-serialises,
  reporting which happened, **so** the user is told when formatting changed.
- `afterSave` clears the recorded edits, **so** repeated saves stay correct.

**Key code.**
- [ui.go](../../internal/ui/ui.go) — `saveFileAs` (the ordering constraint),
  `writeDocumentTo` (splice vs fallback), and `moveFile`.

**Verification.**
```
go test ./internal/ui/ -run 'TestSave|TestSecondSave' -count=1 -v
```
```
--- PASS: TestSaveRewritesOnlyTheEditedBytes
--- PASS: TestSaveWithoutEditsLeavesTheFileByteIdentical
--- PASS: TestSecondSaveSplicesAgainstTheAlreadySavedFile
--- PASS: TestSaveJSONLinesTouchesOnlyTheEditedRow
ok  	github.com/ErikKalkoken/janice/internal/ui	1.375s
```
These write a real file to disk, edit one node through the UI, save, and compare
the bytes — including the second-save case, which splices against the file the
first save produced.

**Notes & open issues.** None.

## Phase 5 — Verification at scale and on awkward input ✅

**Goal.** Establish that byte-fidelity holds on real files and on the shapes a
hand-rolled byte scanner tends to get wrong, not just on tidy fixtures.

🔵 **Core ideas introduced.** None — this phase adds no mechanism, only evidence.

**What changed.**
- Added twelve shape cases covering top-level arrays, empty containers, ten-deep
  nesting, every JSON escape plus non-ASCII, missing trailing newlines, exotic
  number forms and ragged whitespace, **so** the scanner's token handling is
  pinned rather than assumed.
- Added five JSON Lines shape cases (blank lines between rows, CRLF, bare arrays
  as rows, leading whitespace, no trailing newline), **so** the synthetic-root
  path is covered as thoroughly as the single-value one.

**Key code.**
- [splice_test.go](../../internal/jsondocument/splice_test.go) —
  `TestSpliceHandlesAwkwardShapes` and `TestSpliceJSONLinesAwkwardShapes`.

**Verification.**
A generated 4.2 MB document with 299,593 nodes, loaded and then spliced with no
edits, compared byte for byte against the original:
```
nodes=299593 load=73ms splice=36ms bytes=4238876
RESULT: byte-identical
```
All shape cases pass; full suite green:
```
ok  github.com/ErikKalkoken/janice/internal/github
ok  github.com/ErikKalkoken/janice/internal/jsondocument
ok  github.com/ErikKalkoken/janice/internal/ui
```
`go vet ./...` is clean and the Windows GUI binary builds.

**Notes & open issues.**
> [!WARNING]
> 🔴 A document with a JSON scalar at top level (`5`, `"abc"`) still cannot be
> loaded at all — `render` accepts only an object or array. Pre-existing, not
> introduced here, but it is the one input shape the splicer never sees.
>
> 🟡 Structural editing (adding or deleting nodes) is still unsupported. The
> splicer's whole design rests on the tree and the file having the same shape;
> insertion would need node ids that do not exist in the source, so it needs a
> different mechanism than the one built here.
