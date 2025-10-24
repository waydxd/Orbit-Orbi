# Quick Start Guide

Get Orbi up and running in 5 minutes!

## Prerequisites

- Go 1.21 or higher
- OpenAI API key ([get one here](https://platform.openai.com/api-keys))

## Step 1: Clone the Repository

```bash
git clone https://github.com/waydxd/Orbit-Orbi.git
cd Orbit-Orbi
```

## Step 2: Install Dependencies

```bash
go mod download
```

## Step 3: Build Orbi

```bash
make build
```

## Step 4: Start the Example Calendar Service

Open a terminal and run:

```bash
cd examples/calendar-service
go run main.go
```

You should see:
```
Calendar service starting on port 50051...
```

## Step 5: Configure Orbi

Create a `.env` file or set environment variables:

```bash
export CALENDAR_SERVICE_ADDR=localhost:50051
export OPENAI_API_KEY=your-openai-api-key-here
export OPENAI_MODEL=gpt-3.5-turbo
```

## Step 6: Run Orbi

In a new terminal:

```bash
./bin/orbi
```

## Step 7: Start Chatting!

```
You: Create a meeting tomorrow at 2pm titled "Team Sync"
Orbi: I've created a meeting titled "Team Sync" for tomorrow at 2:00 PM.

You: What events do I have?
Orbi: You have 1 event: Team Sync scheduled for tomorrow at 2:00 PM.

You: Delete the Team Sync meeting
Orbi: Event deleted successfully.
```

Type `exit` or `quit` to exit.

## Quick Start with Docker

If you prefer Docker:

```bash
# Build the image
docker build -t orbi:latest .

# Run with environment variables
docker run -e CALENDAR_SERVICE_ADDR=localhost:50051 \
           -e OPENAI_API_KEY=your-api-key \
           -it orbi:latest
```

## Troubleshooting

### "Failed to connect to calendar service"
- Make sure the calendar service is running on port 50051
- Check firewall settings

### "OPENAI_API_KEY not set"
- Set the environment variable before running Orbi
- Get an API key from OpenAI

### Build errors
- Ensure you have Go 1.21 or higher: `go version`
- Run `go mod tidy` to clean up dependencies
- Make sure protoc is installed for regenerating proto files

## Next Steps

- Read the [README.md](README.md) for detailed documentation
- Check [IMPLEMENTATION.md](IMPLEMENTATION.md) for architecture details
- Explore the [example calendar service](examples/calendar-service/README.md)
- Customize the agent in `pkg/orbi/agent.go`
- Add your own tools and capabilities

## Need Help?

- Open an issue on GitHub
- Check the documentation in README.md
- Review the example code in `examples/`

Happy scheduling! 🚀
