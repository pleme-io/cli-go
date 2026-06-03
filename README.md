# cli-go

The pleme-io **CLI framework** for Go — the Go counterpart to the Rust
[`clap`](https://github.com/clap-rs/clap) / `caixa-clap` model. A named app, a
tree of subcommands, per-flag validators, and a small multi-auth resolver — so
every Go binary in the fleet dispatches, validates, and prints usage the same
way.

> **No hand-rolled `argv` switch statements, no bespoke flag parsing per tool.**
> Build an `App`, `Add` commands, hand it `os.Args`.

## Why

Stdlib `flag` parses flags but stops there: no subcommands, no validators, no
usage tree, no auth-selection convention. cli-go adds exactly those layers —
and nothing else. The **core package is pure standard library, zero
dependencies** (`flag`, `context`, `errors`, `os`, `reflect`, `sort`,
`strconv`, `strings`), so it builds offline with a minimal closure. The
dep-bearing integrations live in leaf sub-packages (`cli-go/cfg`, `cli-go/exit`)
so the core stays weightless (Borealis Laws 6 & 8).

## Borealis elevation (§2.2)

Under the [Borealis Theory](../theory/BOREALIS.md), cli-go is the fleet CLI
**authoring surface**: it owns the typed *grammar* (a single source of truth),
while *presentation* is owned by `borealis`. Help **structure** — flag types,
enum sets, defaults, env names, the command listing — is **DERIVED** from the
typed data and is never hand-formatted (Law 4). The elevated surface adds,
additively over the original:

- **`Flag[T]` — validation-as-data.** A generic, self-describing flag whose
  `Name`, `Default`, `Env`, `OneOf`, and typed `Validate` travel together.
  `OneOf` both validates *and* auto-documents the allowed set (the kong `enum:`
  model). Read a parsed, validated, typed value with `Flag.Get()`.
- **`FleetFlags` — the persistent flag set, defined once.** `--auth` (its value
  set auto-wired from `AuthResolver.Kinds()`), `--profile`, `--gateway-url`,
  `--output json|yaml` (`table` opt-in), `--no-color`/`NO_COLOR`, `--verbose`.
- **Raised `Command`.** `Aliases`, `Hidden`, `Category`, `Deprecated` (derived
  into help) plus authored prose `Long` / `Examples` (typed data, rendered).
- **`cli-go/cfg` — config (Law 3, §3.5).** `cfg.WithConfig` internally calls
  `shikumi.For`, runs the canonical precedence pipeline (args > env > file),
  selects the primitive's sub-struct, and hands it to the consumer's
  `FromConfig`. One loader, one consumer shape, wired in this leaf — never a
  second loader idiom.
- **`cli-go/exit` — exit codes (errors-go is the sole owner, §3.5).** `exit.Map`
  maps `ErrHelp` → 0 and usage errors (`ErrNoCommand`, `*ValidationError`,
  unknown command/subcommand, parse failures) → `EX_USAGE` via
  `errors.WithExitCode`. It never calls `os.Exit`; the single funnel is
  `errs.Exit(run())` at `main`.

> **Deferred (in-flight siblings):** `borealis.Execute(ctx, root, opts…)` is the
> CLI entrypoint, owned by `borealis` (which pre-wires the color scheme); the
> root `Command` defined here is *passed to* it — cli-go does **not** redeclare
> `Execute`. Borealis-rendered styled help/errors are likewise deferred until
> `borealis.Execute`/`Render` lands. The core builds and tests green standalone.

## Model

- **App** — a named root with a version and description (functional options:
  `WithVersion`, `WithDescription`, `WithOutput`). Built-in `--help`/`-h` and
  `--version`/`-v`.
- **Command** — `{Name, Summary, Flags, Run, Sub}`. `Flags` registers onto a
  fresh per-invocation `*flag.FlagSet`; `Run` receives the parsed, validated set
  and the positional args. `Sub` enables nested subcommands (≥ 1 level); a
  parent with its own `Run` acts as the default when no child matches.
- **Validators** — register a `Validator` per flag with `RegisterValidator`
  inside the `Flags` callback. They run after `Parse`, before `Run`, and the
  first failure returns a typed `*ValidationError`. Bundled constructors:
  `Required`, `OneOf`, `Range`, `NonEmptyURL`, `Predicate`.
- **Multi-auth** — `AuthResolver` selects exactly one `AuthMethod` from a
  registered set (api-key / token / aws-iam / azure-ad / gcp / k8s / …) via an
  explicit `--auth` selector or env-var auto-detection. Transport-free: it
  resolves *which* method and *what* inputs, not how to call any server.

## Usage

```go
app := cli.NewApp("clint",
    cli.WithVersion("5.0.22"),
    cli.WithDescription("Akeyless CLI"),
)

app.Add(cli.Command{
    Name:    "list-secrets",
    Summary: "List secrets under a path",
    Flags: func(fs *flag.FlagSet) {
        fs.String("path", "/", "secrets path")
        fs.Int("limit", 100, "max results")
        cli.RegisterValidator(fs, "path", cli.Required())
        cli.RegisterValidator(fs, "limit", cli.Range(1, 1000))
    },
    Run: func(ctx context.Context, args []string, fs *flag.FlagSet) error {
        // fs is parsed and validated here.
        return nil
    },
})

// Nested: `clint auth login`
app.Add(cli.Command{
    Name:    "auth",
    Summary: "Authentication commands",
    Sub: []cli.Command{
        {Name: "login", Summary: "Log in", Run: doLogin},
    },
})

if err := app.Run(context.Background(), os.Args); err != nil &&
    !errors.Is(err, cli.ErrHelp) {
    log.Fatal(err)
}
```

### Typed flags + fleet flags (elevated)

```go
// Validation-as-data: name/default/env/OneOf/Validate travel together, and the
// allowed set auto-documents in --help (Law 4).
mode := cli.NewFlag[string]("mode", "fast", "execution mode").
    Env("TOOL_MODE").
    OneOf("fast", "slow")

port := cli.NewFlag[int]("port", 8080, "listen port").
    Validate(func(v int) error {
        if v < 1 || v > 65535 { return errors.New("out of range") }
        return nil
    })

// The fleet-standard persistent flag set — defined once; --auth's set is
// auto-wired from the resolver's Kinds().
fleet := cli.NewFleetFlags(resolver, cli.AllowTable())

app.Add(cli.Command{
    Name:     "run",
    Aliases:  []string{"r"},
    Category: "Core",
    Long:     "Runs the job against the selected gateway.",
    Examples: []string{"clint run -mode fast --output json"},
    Flags: func(fs *flag.FlagSet) {
        mode.Bind(fs)
        port.Bind(fs)
        fleet.Bind(fs)
    },
    Run: func(ctx context.Context, args []string, fs *flag.FlagSet) error {
        _ = mode.Get()             // typed, validated read
        _ = fleet.GatewayURL.Get()
        return nil
    },
})
```

### Config (`cli-go/cfg`) and exit codes (`cli-go/exit`)

```go
// One loader (shikumi.For), one consumer (FromConfig), wired in the cfg leaf.
build := cfg.WithConfig(
    "akeyless-foo",
    Root{},                                            // typed defaults
    func(r Root) logging.Config { return r.Logging },  // select sub-struct
    logging.FromConfig,                                // primitive's FromConfig
    cfg.WithEnvPrefix("AKL_FOO_"),
)
log, err := build(ctx)

// errors-go owns process exit; cli-go/exit only maps sentinels to codes.
func main() { errs.Exit(run()) }                       // the single funnel
func run() error {
    err := app.Run(context.Background(), os.Args)
    return exit.Map(err)                               // ErrHelp→0, usage→EX_USAGE
}
```

### Multi-auth selection

```go
r := cli.NewAuthResolver().
    Register(cli.AuthMethod{Kind: cli.AuthAPIKey, EnvVar: "AKEYLESS_ACCESS_KEY"}).
    Register(cli.AuthMethod{Kind: cli.AuthToken,  EnvVar: "AKEYLESS_TOKEN"}).
    Register(cli.AuthMethod{
        Kind:   cli.AuthAWS,
        EnvVar: "AWS_ROLE",
        Resolve: func(role string) (map[string]string, error) {
            return map[string]string{"role": role, "region": "us-east-1"}, nil
        },
    })

// Explicit --auth flag, or "" to auto-detect from whichever env var is present.
res, err := r.Resolve(authFlagValue)
// res.Kind, res.Credentials
```

## Sentinel errors

`App.Run` returns `ErrHelp` when help was printed and `ErrNoCommand` when argv
carried no subcommand — treat both as clean exits. A flag validator failure
returns a `*ValidationError` (use `errors.As`); it carries the offending `Flag`
and `Value` and unwraps to the validator's own error.

## Build & test

```bash
go build ./...
go test ./...
```

The **core package** is pure standard library — nothing to vendor. The
`cli-go/cfg` and `cli-go/exit` leaf sub-packages depend on the elevated siblings
`shikumi-go` and `errors-go` respectively; during pre-publish development those
resolve via temporary local `replace` directives in `go.mod` (removed at
publish).
