# Hooks Library

`hooks/*.example.sh` are templates. They do not run until copied to a non-example filename.

## Enable Hooks

```bash
cp hooks/homebrew.example.sh hooks/01-homebrew.sh
cp hooks/mise.example.sh hooks/02-mise.sh
cp hooks/apt.example.sh hooks/01-apt.sh
```

## Package Manifests

- `Brewfile` for Homebrew packages
- `Aptfile` for apt packages
- `mise.toml` for runtimes (create manually)

`Brewfile.example` includes examples for `tap`, `brew`, and `cask` entries.

## Add A New Hook

1. Copy `hooks/hook.example.sh` to an ordered filename like `hooks/50-my-tool.sh`.
2. Source `hooks/lib.example.sh` to reuse helper functions.
3. Handle `pre-link`, `post-link`, and `status` as needed.
4. `chmod +x hooks/50-my-tool.sh`.
