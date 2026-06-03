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
and nothing else. **Pure standard library, zero dependencies** (`flag`,
`context`, `errors`, `sort`, `strings`), so it builds offline with a minimal
closure.

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

Pure standard library — nothing to vendor.
