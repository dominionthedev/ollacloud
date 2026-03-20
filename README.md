# ollacloud

**Ollama-compatible cloud proxy.** Use your Ollama Cloud subscription without downloading Ollama or any models locally.

`ollacloud` runs a fully wire-compatible Ollama HTTP server on `localhost:11434`. Any app, script, or tool already built for Ollama — Open WebUI, Cursor, Cline, Continue, custom scripts — works with zero changes.

---

## Why

| Problem | ollacloud solution |
|---|---|
| Ollama binary is large | Single 9 MB static binary |
| Models require gigabytes of local storage | Models live in your cloud account |
| VRAM required for local inference | Inference runs on Ollama Cloud GPUs |
| Apps hardcoded to `localhost:11434` | Same port, same API — drop-in replacement |

---

## Quick start

```sh
# 1. Set your API key (get one at https://ollama.com/settings/keys)
export OLLAMA_API_KEY=your_key_here

# 2. Start the daemon (binds to localhost:11434)
ollacloud serve

# 3. In another terminal — use it exactly like Ollama
ollacloud run gemma3
ollacloud pull deepseek-v3:671b-cloud
ollacloud list
```

Any existing Ollama client now talks to the cloud:

```sh
curl http://localhost:11434/api/generate -d '{
  "model": "gemma3",
  "prompt": "Why is the sky blue?"
}'
```

---

## Auth

ollacloud resolves your API key in priority order:

```
--key flag  →  OLLAMA_API_KEY env  →  config file  →  interactive prompt
```

To persist a key:

```sh
ollacloud auth set       # prompts and saves to ~/.config/ollacloud/config.toml
ollacloud auth status    # show where the current key is coming from
ollacloud auth remove    # clear the stored key
```

---

## Commands

| Command | Description |
|---|---|
| `serve` | Start the daemon on `:11434` |
| `run <model>` | Interactive Bubbletea TUI chat session |
| `pull <model>` | Pull a model to your cloud account |
| `push <model>` | Push a model to the Ollama registry |
| `list` / `ls` | List models in your cloud account |
| `ps` | List models with active requests |
| `show <model>` | Show model info card |
| `rm <model>` | Remove a model from your cloud account |
| `cp <src> <dst>` | Copy a model to a new name |
| `create <model>` | Create a model from a Modelfile |
| `stop <model>` | No-op (cloud manages its own resources) |
| `auth set/remove/status` | Manage API key |
| `version` | Print version |

### Interactive session slash commands

Inside `ollacloud run`:

```
/exit, /bye          Quit the session
/clear               Clear conversation history
/set system <text>   Set the system prompt
/set parameter temperature <f>
/set parameter num_ctx <n>
/set parameter top_p <f>
/show info           Model and connection info
/show parameters     Current session parameters
"""                  Toggle multiline input mode
PgUp / PgDn          Scroll history
```

---

## How it works

```
Your app / ollacloud CLI
        │ Ollama API (no auth required locally)
        ▼
ollacloud daemon  (:11434)
        │ Injects:  Authorization: Bearer $OLLAMA_API_KEY
        │ Rewrites: host → https://ollama.com
        ▼
  Ollama Cloud API
```

The proxy is completely transparent. Streaming NDJSON responses are pumped token-by-token with immediate `Flush()` calls — perceived latency is identical to talking directly to the cloud.

---

## Configuration

Config file: `~/.config/ollacloud/config.toml`

```toml
api_key      = "your_key_here"
upstream_url = "https://ollama.com"   # default
port         = 11434                   # default
```

All fields are optional — sensible defaults apply with no config file present.

---

## Differences from Ollama

| Feature | Ollama | ollacloud |
|---|---|---|
| Local inference | ✅ | ❌ (by design) |
| Cloud inference | ✅ (with subscription) | ✅ |
| `pull` behaviour | Downloads model locally | Registers model in cloud account |
| `stop` | Unloads from VRAM | No-op (cloud manages resources) |
| `ps` VRAM sizes | Accurate | Reports zero (no local VRAM) |
| Binary size | ~500 MB | ~9 MB |
| Model storage | Local disk (GBs) | Cloud account |

---

## Building from source

```sh
git clone https://github.com/dominionthedev/ollacloud
cd ollacloud
make build
./bin/ollacloud --help
```

Requires Go 1.22+.

