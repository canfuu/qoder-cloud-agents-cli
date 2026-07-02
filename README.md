# qca - Qoder Cloud Agents CLI

A command-line interface for managing [Qoder Cloud Agents](https://docs.qoder.com/cloud-agents/overview). Wraps the official API into a familiar CLI experience with authentication similar to `gh`.

## Installation

```bash
go install github.com/canfuu/qoder-cloud-agents-cli@latest
```

Or build from source:

```bash
git clone https://github.com/canfuu/qoder-cloud-agents-cli.git
cd qoder-cloud-agents-cli
go build -o qca .
```

## Authentication

Login with a Personal Access Token (similar to `gh auth login`):

```bash
# Interactive
qca auth login

# Non-interactive
qca auth login --token <your-pat>

# Or use environment variable
export QODER_PAT="your-personal-access-token"
```

Check status:
```bash
qca auth status
```

## Commands

### Agents

```bash
qca agent list                           # List all agents
qca agent list --json                    # Output as JSON
qca agent get <agent-id>                 # Get agent details
qca agent create -n my-agent             # Create agent with defaults
qca agent create -n my-agent -m ultimate -s "You are helpful." --tools Bash,Read,Write
qca agent update <agent-id> -n new-name  # Update agent
```

### Environments

```bash
qca env list                             # List all environments
qca env get <env-id>                     # Get environment details
qca env create -n my-env                 # Create with unrestricted networking
qca env create -n secure --networking limited
qca env update <env-id> --networking unrestricted
```

### Sessions

```bash
qca session list                         # List all sessions
qca session get <session-id>             # Get session details
qca session create -a <agent-id> -e <env-id>  # Create session
qca session send <session-id> "Hello"    # Send a message
qca session stream <session-id>          # Stream events (SSE)
qca session events <session-id>          # List event history
qca session cancel <session-id>          # Cancel current turn
qca session archive <session-id>         # Archive session
qca session delete <session-id>          # Delete session
qca session chat <session-id>            # Interactive chat mode
```

### Models

```bash
qca model                                # List available models
```

## Configuration

Config is stored at `~/.config/qca/config.json`:

```json
{
  "token": "your-pat-token",
  "api_base": "https://api.qoder.com/api/v1/cloud"
}
```

You can also use environment variables:
- `QODER_PAT` - Personal Access Token
- `QODER_API_BASE` - Custom API base URL

## License

MIT
