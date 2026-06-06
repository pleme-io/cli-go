;; caixa.lisp — the single source of truth for cli-go's kind + ecosystem.
;;
;; Consumed by `pleme-doc-gen` for the SDLC pipeline (flake.nix +
;; .pleme-io-release.toml + auto-release workflow + nix module trio).
;; Re-emit the generated surface with:
;;   pleme-doc-gen caixa --source caixa.lisp --out . --force
;;
;; NOTE: the authored Go source + go.mod are NOT regenerated. The render
;; adds release scaffolding only; go.mod is protected across re-render.

(defcaixa cli-go
  :kind         :Biblioteca
  :ecosystem    :go

  :package      { :name        "cli-go"
                  :version     "0.1.0"
                  :license     "MIT"
                  :description "pleme-io's CLI framework for Go — the counterpart to the Rust clap / caixa-clap model: named app, subcommand tree, per-flag validators, multi-auth resolver."
                  :module-path "github.com/pleme-io/cli-go"
                  :repository  "https://github.com/pleme-io/cli-go"
                  :homepage    "https://github.com/pleme-io/cli-go"
                  :go-version  "1.25" }

  :supports     { :go ">=1.25" }

  :ci-config    { :bump    { :default-type "patch" }
                  :publish { :no-verify false } }

  :workflows    [ :auto-release ]
  :stacks       [ ]
  :depends-on   [ ]
  :exposes      [ :go-module ]
  :publish-to-git true)
