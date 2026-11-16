# Testing

## How it works

Start with these focused tests:

1. **Pure rules get table-driven tests.** A dice game's `domain/scoring.go` is 15
   scoring functions over plain dice values — no ebiten in the file, no world, no
   rendering — and `scoring_test.go` (369 lines) covers every one with a table.
2. **Parsers and loaders get golden-input tests.** A Twine parser
   (`assets/twine/parser.go`, stdlib + `domain` only) feeds a realistic passage string in
   and asserts the entire parsed struct in one `assert.Equal`.

A tabletop tactics game extends the pattern with tests of contracts across whole
battles and comparisons against simpler reference algorithms. These fit the same pure
domain boundary;
they do not require a window or an ECS scene. Test presentation seams when they
have meaningful behavior to verify, and exercise rendering and interaction in the
running game.

## Canonical code

**A rule function** — pure data in, score out (`domain/scoring.go`):

```go
func diceCounts(dice []uint) map[uint]int {
    counts := map[uint]int{}
    for _, d := range dice {
        counts[d]++
    }
    return counts
}

func twoPairs(dice []uint) uint {
    counts := diceCounts(dice)

    var scores []int

    for k, v := range counts {
        if v >= 2 {
            scores = append(scores, int(k)*2)
        }
    }

    if len(scores) < 2 {
        return 0
    }

    sort.Ints(scores)

    return uint(scores[len(scores)-1] + scores[len(scores)-2])
}
```

**Its test** — testify, a case table, subtests named from the input
(`domain/scoring_test.go`, trimmed to three cases):

```go
func TestTwoPairs(t *testing.T) {
    testCases := []struct {
        Dice          []uint
        ExpectedScore uint
    }{
        {
            Dice:          []uint{1, 2, 3, 4, 5},
            ExpectedScore: 0,
        },
        {
            Dice:          []uint{1, 1, 2, 2, 5},
            ExpectedScore: 6,
        },
        {
            Dice:          []uint{6, 5, 5, 6, 5},
            ExpectedScore: 22,
        },
    }

    for _, tc := range testCases {
        t.Run(testName(tc.Dice), func(t *testing.T) {
            score := twoPairs(tc.Dice)
            assert.Equal(t, tc.ExpectedScore, score)
        })
    }
}
```

`testName` renders the input as the subtest name, so a failure reads
`TestTwoPairs/6_5_5_6_5` and `-run 'TestTwoPairs/6_5_5_6_5'` replays it:

```go
func testName(values []uint) string {
    var s []string
    for _, v := range values {
        s = append(s, strconv.Itoa(int(v)))
    }
    return strings.Join(s, "_")
}
```

**A golden parser test** — real input inline, whole output struct asserted
(`assets/twine/parser_test.go`, trimmed):

```go
func TestParsePassage(t *testing.T) {
    passage := `addItem: key
takeMoney: 100
--
[h1]
Actual title
[continue]

First line.

Second line.
And third line.

[if hasItem key]
Only if has key.
[continue]
` // … the real input continues: else-branches, && conditions, conditional links

    parsed := parsePassage("This is title [tag1 tag2]", passage)

    expectedPassage := domain.RawPassage{
        Title: "This is title",
        Tags:  []string{"tag1", "tag2"},
        Paragraphs: []domain.Paragraph{
            {
                Text: "Actual title",
                Type: domain.ParagraphTypeHeader,
            },
            // … every paragraph, macro, and link spelled out
        },
    }

    assert.Equal(t, expectedPassage, parsed)
}
```

Three such tests cover the whole passage grammar (see [dialogue](dialogue.md)). When the
format grows a feature, paste a real passage using it and spell out the struct.

## Complete-game contracts

`TestPendingAndAdviceAreLegal` (`domain/options_test.go`) runs a seeded CPU
battle and checks the public decision contract at every step: the decision names
the right team, every listed option and advised move passes `Preview`, relevant
decisions have an answer, and the battle reaches its result within a step limit.
It records the kinds of decisions visited and checks coverage of the expected
phases, so a shortened battle cannot silently skip the feature being tested.

Use the same `Execute` and read-model events as the game. In addition to the chosen
command, check the alternatives advertised to a player or seat: those are promises
made by the interface even when the CPU never chooses them. For multi-step plans,
this test verifies the first step's legality; future steps remain conditional and
must be revalidated when played.

Fix the seed and cap the number of steps so a regression is reproducible and a
stalled battle fails. Long full-battle tests may skip under `testing.Short`; keep
small tests for individual rules and regressions too. A completed CPU battle is
evidence about the states it visited, not exhaustive proof of every legal game.

## Reference algorithms and asymmetric guarantees

Optimized geometry needs more than examples that duplicate its implementation.
`domain/reach_test.go` uses a slower dense-sample search
that tries increments between every pair of sampled points. It shares the domain's
exact segment legality/cost primitive but independently searches routes, so it can
catch search errors without inventing another set of movement rules. Separate
geometry tests cover that shared primitive.

Check the guarantee that matters to the player:

- **Reach:** sampled advertised destinations have witness routes accepted by
  `Preview`. A separate tightness check permits missing at most 1% of the oracle's
  legal sampled destinations in its fixture.
- **Danger:** on seeded randomized boards, every target the oracle can charge must
  overlap the danger area. A missed threat is a false promise of safety.

These are different error directions: suggested movement must be achievable, while
a safety display must not omit known threats. State the sampling and tolerance
explicitly. The oracle samples a finite set; it does not prove continuous geometry
correct everywhere.

## Equivalence and snapshot isolation

Compare two ways to reach an outcome when exact intermediate state is unimportant.
`TestUndoReplaysIdentically` in `domain/undo_test.go`
compares move → shoot with move → undo → same move → shoot and requires the same
dice. Other tests assert that rolls and confirmed allocations block undo. Together
these catch accidental RNG consumption and missing commitment boundaries.

Order-revision tests in `domain/activation_test.go`
check that a legal replay preserves movement and AP, while a revision that makes a
previous Charge illegal is refused. Snapshot tests should also mutate returned
nested data and verify that authoritative state and prior events remain detached;
checking only the top-level struct misses shared slices and pointers.

## Headless arena

`cmd/arena/main.go` runs the same `Seat`/`Command` contract to
completion without presentation delays. Its integration test loads real boards
through the Ebitengine-free `assets/boards` subpackage and checks that CPU play
reaches `Results`. The arena also reports scores and behavior over boards, missions,
and seeds, making it useful for evaluating planner changes beyond one playthrough.

Keep correctness checks and balance measurements distinct: a seat must always send
legal commands and finish, but a particular winner is usually not a stable test
expectation. CPU-only arena runs require no model calls. AI arena runs use the
configured provider and spend credit; they are a separate evaluation, not a dependency
of the ordinary test suite.

## Rules

- **Every `domain/` package and every parser gets tests from day one.** These are the
  packages that are pure by construction ([domain](domain.md)) — the test costs minutes,
  and a pure rules layer without tests is the cheapest coverage left on the table.
- **Headless rules and loader packages must not import ebiten, even transitively.**
  Other tested packages may use it, but then their tests need its native dependencies
  and runtime environment. A pure test file in a rendering package does not make the
  whole package headless (see Commands).
- ECS/render systems are exercised by playing, not unit-tested — until a real bug proves
  a need, then test that seam and no more.
- Table-driven with testify `assert`/`require`; subtest names derived from the case input
  (`testName`), not hand-written labels.

## Commands

```sh
go test ./...                    # everything
go test -run TestTwoPairs ./domain   # one test, one package
```

Caveat: on a machine without ebiten's C deps (X11/OpenGL
headers — e.g. the dev container), native builds of ebiten-importing packages fail, and
vet must run as `GOOS=js GOARCH=wasm go vet ./...`. `go test ./...` compiles every
package, so in that environment run tests per pure package (`go test ./domain/`) — which
works because those selected packages never touch ebiten. This restriction also
applies to pure math tests colocated with renderer files (a `dice` package that holds
both physics and drawing).

## Pitfalls

- Rules that score `[]*entity.Die`, where package `entity` also defines an
  ebiten-importing `DieIcon`, pull ebiten into the tests transitively. Keep rules in
  `domain/` over plain values (`[]uint`) and the leak can't happen.
