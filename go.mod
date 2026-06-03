module github.com/pleme-io/cli-go

go 1.25.0

// TEMP local replaces for already-committed elevated siblings (removed at
// publish). The cli-go CORE imports neither: shikumi-go is imported only by the
// cli/cfg leaf (Law 8) and errors-go only by the cli/exit leaf.
require (
	github.com/pleme-io/errors-go v0.0.0
	github.com/pleme-io/shikumi-go v0.0.0
)

require (
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/parsers/json v1.0.0 // indirect
	github.com/knadh/koanf/parsers/toml v0.1.0 // indirect
	github.com/knadh/koanf/parsers/yaml v1.1.0 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/providers/file v1.2.1 // indirect
	github.com/knadh/koanf/v2 v2.3.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/sethvargo/go-envconfig v1.3.0 // indirect
	go.yaml.in/yaml/v3 v3.0.3 // indirect
	golang.org/x/sys v0.45.0 // indirect
)

replace github.com/pleme-io/shikumi-go => ../shikumi-go

replace github.com/pleme-io/errors-go => ../errors-go
