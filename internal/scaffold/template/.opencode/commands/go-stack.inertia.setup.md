---
description: Instructions to setup the go-stack Inertia.js integration for fullstack
  Go applications, including boot provider wiring and frontend workflow.
---

# Setup Inertia.js Integration for `go-stack`

## 1. Install the `pkg/inertia` Go module

Add it to your project's `go.mod`:

```bash
go get github.com/emilioforrer/go-stack/pkg/inertia@latest
```

## 2. Setup frontend scaffold into your project

- Get the path of the inertia package by running `MODPATH=$(go list -m -f '{{.Dir}}' github.com/emilioforrer/go-stack/pkg/inertia)`
- Copy the `$MODPATH/resources` directory from the inertia package into your project's root directory if not exists.
- Copy the `$MODPATH/vite.config.js` file into your project's root directory if not exists.
- Copy the `$MODPATH/ssr.config.js` file into your project's root directory if not exists.
- Copy the `$MODPATH/package.json` file into your project's root directory if not exists.
- Copy the `$MODPATH/package-lock.json` file into your project's root directory if not exists.
- Create a `public` directory in your project's root directory if not exists, and add `.gitkeep` file inside it.
- Create a `bootstrap` directory in your project's root directory if not exists
- Add to the `.gitignore` your project's root directory
```
# Inertia scaffold
node_modules
public/build
public/hot
bootstrap
.vite
```
- Add to the `Taskfile.yml` your project's root directory
```yaml
tasks:
  npm:install:
    desc: Install Node dependencies
    cmds:
      - npm install

  npm:dev:client:
    desc: Start Vite dev server
    cmds:
      - npm run dev:client

  npm:dev:ssr:build:
    desc: Build SSR bundle in watch mode
    cmds:
      - npm run dev:ssr:build

  npm:ssr:
    desc: Start SSR server
    cmds:
      - npm run ssr

  npm:build:node:
    desc: Build client and SSR bundles
    cmds:
      - npm run build
```
- Run `task npm:install` in your project's root directory to install the frontend dependencies.
- Run `go mod tidy` in your project's root directory to clean up your `go.mod` file.

## 3. Wire the Inertia provider into your boot setup
In your `cmd/app/serve.go` file, import the inertia package and add the inertia provider to your boot setup:


```go
import (
	"github.com/emilioforrer/go-stack/pkg/inertia"
)
func runServe(ctx context.Context, _ *cobra.Command) error {
  // ... your existing code ...
  bootstrapper := newBootstrapper(
    i,
    provider.NewServerProvider(opts),
    // Add the Inertia provider with your desired options, after the ServerProvider
    inertia.NewInertiaProvider(inertia.ProviderOptions{
      Enabled:       true,
      Env:           "dev",
      SSREnabled:    false,
      PublicPath:    "public",
      ResourcesPath: "resources",
    }),
  )
  // ... your existing code ...
}
```

## Additional References

- [Inertia.js Documentation](https://inertiajs.com/)
- [GoNertia (Go Inertia adapter)](https://github.com/romsar/gonertia)
- [Vite Documentation](https://vitejs.dev/)
- [go-stack boot package](https://github.com/emilioforrer/go-stack/tree/main/pkg/boot)