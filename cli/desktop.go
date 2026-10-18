package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
)

const desktopUsage = `Work with projects and tasks in an open Glowbom Desktop session.

Usage:
  glowbom desktop project current [--json]
  glowbom desktop models list [--json]
  glowbom desktop chat send --message TEXT [--project ABS_PATH] [--context ID] [--model ID] [--actor USER]
  glowbom desktop build start --instructions TEXT [--project ABS_PATH] [--context ID] [--model ID] [--actor USER]
  glowbom desktop runs show RUN_ID [--json]
  glowbom desktop agents list [--json]
  glowbom desktop agents register --username USER --name NAME [--provider NAME]
  glowbom desktop agents whoami --agent USER [--json]
  glowbom desktop tasks list [--agent USER] [--json]
  glowbom desktop tasks create --agent USER --title TITLE --instructions TEXT
  glowbom desktop tasks next --agent USER [--json]
  glowbom desktop tasks show TASK_ID [--json]
  glowbom desktop tasks reply TASK_ID --agent USER --message TEXT
  glowbom desktop tasks submit TASK_ID --agent USER --attempt ATTEMPT_ID --revision N --message TEXT
  glowbom desktop tasks accept TASK_ID --result RESULT_ID [--agent USER] [--revision N]
  glowbom desktop files get FILE_ID --output NEW_PATH [--json]

Options:
  --json                     Print one compact JSON response
  --instructions-file PATH   Read task or build instructions from a UTF-8 file
  --project VALUE            Task label, or absolute project path for chat/build
  --context ID               Pin chat/build to the context from project current
  --model ID                 Choose a model reported by models list
  --message-file PATH        Read chat, reply, or result text from a UTF-8 file
  --file PATH                Attach a file to create, reply, or submit; may repeat
  --attempt ATTEMPT_ID        Use the attempt returned by next
  --revision N               Use the task revision returned by next or show
  --actor USER               Override the author name for attribution
  --request-id ID            Reuse this ID when retrying the same mutation
  --output NEW_PATH          Save an attachment without replacing an existing file

Open Tasks in a signed-in Glowbom Desktop session first. These commands use
that local session. Agent names describe authors and select work; they do not
grant separate access. The CLI does not open Desktop or run an external agent.

For project commands, keep Chat open in Desktop. Build also needs an open
project. Chat and Build use the open project. --project checks its path without
switching projects. --context rejects a request if the conversation changed.
Use --model to override the selected model for this request.
Send and start return a run ID; use runs show to read its progress and result.
Agent polling schedules remain external. Chat and Build do not accept --file.

Create assigns work to --agent and uses the owner as author unless --actor is
given. Next returns a task with its complete history,
attempt ID, and revision. When there is no work, it returns {"task":null} and
exits successfully. Submit includes the attempt and revision from next.
Accept selects one result. Everyone using the local session can create or accept.
To resume after a disconnect, use tasks list --agent USER, then tasks show
TASK_ID to recover the current attempt and history. Next claims queued work;
it does not start a second attempt for work already in progress.

Attachments send file bytes, never local paths. Each file may be up to 8 MiB,
with at most 8 files and 16 MiB combined. Text files may be up to 128 KiB.
Replies always use JSON; without --json they are indented for reading.

If several Desktop sessions are open, set GLOWBOM_DESKTOP_CONNECTION_FILE to
the absolute path of the chosen private connection record under
~/.glowbom/desktop/tasks-connections/. Never share that file or its token.
`

type desktopInvocation struct {
	command desktopCommand
	compact bool
	help    bool
	output  string
}

func runDesktopCommand(args []string) int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return runDesktopWithIO(ctx, args, os.Stdout, os.Stderr)
}

func runDesktopWithIO(ctx context.Context, args []string, out, errOut io.Writer) int {
	invocation, err := parseDesktopCommand(args)
	if err != nil {
		writeDesktopFailure(out, errOut, hasDesktopJSONFlag(args), "invalid_arguments", err.Error(), "")
		return 2
	}
	if invocation.help {
		fmt.Fprint(out, desktopUsage)
		return 0
	}
	client, err := discoverDesktopClient(ctx)
	if err != nil {
		writeDesktopFailure(out, errOut, invocation.compact, "desktop_unavailable", err.Error(), invocation.command.RequestID)
		return 1
	}
	response, err := client.call(ctx, invocation.command)
	if err != nil {
		writeDesktopFailure(out, errOut, invocation.compact, "desktop_request_failed", err.Error(), invocation.command.RequestID)
		return 1
	}
	if !response.OK && response.Error != nil && response.Error.Code == "request_id_required" {
		switch invocation.command.Operation {
		case "project.current", "models.list", "runs.show":
			response.Error = &desktopError{
				Code:    "desktop_update_required",
				Message: "This Desktop version does not support project commands. Use an updated Desktop app or set GLOWBOM_DESKTOP_CONNECTION_FILE to the current development connection.",
			}
		}
	}
	if response.OK && invocation.output != "" {
		response.Result, err = saveDesktopAttachment(ctx, response.Result, invocation.command.Payload["id"].(string), invocation.output)
		if err != nil {
			writeDesktopFailure(out, errOut, invocation.compact, "file_save_failed", err.Error(), invocation.command.RequestID)
			return 1
		}
	}
	if err := writeDesktopResponse(out, response, invocation.compact); err != nil {
		fmt.Fprintln(errOut, "could not write the Desktop response")
		return 1
	}
	if !response.OK {
		return 1
	}
	return 0
}

func writeDesktopResponse(out io.Writer, response desktopResponse, compact bool) error {
	encoder := json.NewEncoder(out)
	if !compact {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(response)
}

func writeDesktopFailure(out, errOut io.Writer, compact bool, code, message, requestID string) {
	if compact {
		_ = writeDesktopResponse(out, desktopResponse{
			Version: 1, Error: &desktopError{Code: code, Message: message}, RequestID: requestID,
		}, true)
		return
	}
	fmt.Fprintln(errOut, message)
	if requestID != "" {
		fmt.Fprintf(errOut, "Request ID: %s\n", requestID)
	}
}

func hasDesktopJSONFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return false
}

func parseDesktopCommand(args []string) (desktopInvocation, error) {
	invocation := desktopInvocation{command: desktopCommand{Version: 1, Payload: map[string]any{}}}
	if len(args) == 0 || args[0] == "help" {
		invocation.help = true
		return invocation, nil
	}
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			invocation.help = true
			return invocation, nil
		}
	}
	if len(args) < 2 {
		return invocation, errors.New("choose a Desktop command; see glowbom desktop --help")
	}
	operation := args[0] + "." + args[1]
	if isDesktopProjectCommand(args[0]) {
		return parseDesktopProjectCommand(operation, args[2:])
	}
	allowed := map[string]string{
		"agents.list":     "",
		"agents.register": "username name provider",
		"agents.whoami":   "agent",
		"tasks.list":      "agent",
		"tasks.create":    "agent title instructions instructions-file project file",
		"tasks.next":      "agent",
		"tasks.show":      "",
		"tasks.reply":     "agent message message-file attempt revision file",
		"tasks.submit":    "agent message message-file attempt revision file",
		"tasks.accept":    "agent result revision",
		"files.get":       "output",
	}
	flagList, ok := allowed[operation]
	if !ok {
		return invocation, errors.New("unknown Desktop command; see glowbom desktop --help")
	}
	flags := map[string]string{}
	for _, name := range strings.Fields(flagList + " json actor request-id") {
		flags[name] = ""
	}
	values := map[string]string{}
	var files, positionals []string
	for index := 2; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "--") {
			positionals = append(positionals, arg)
			continue
		}
		name, value, supplied := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		if _, ok := flags[name]; !ok {
			return invocation, fmt.Errorf("unsupported option --%s; see glowbom desktop --help", name)
		}
		if name == "json" {
			if supplied {
				return invocation, errors.New("use --json without a value")
			}
			invocation.compact = true
			continue
		}
		if !supplied {
			index++
			if index >= len(args) || strings.HasPrefix(args[index], "--") {
				return invocation, fmt.Errorf("--%s requires a value", name)
			}
			value = args[index]
		}
		if strings.TrimSpace(value) == "" {
			return invocation, fmt.Errorf("--%s requires a nonempty value", name)
		}
		if name == "file" {
			files = append(files, value)
			continue
		}
		if _, exists := values[name]; exists {
			return invocation, fmt.Errorf("--%s may only be given once", name)
		}
		values[name] = value
	}
	needsTaskID := operation == "tasks.show" || operation == "tasks.reply" || operation == "tasks.submit" || operation == "tasks.accept"
	if needsTaskID || operation == "files.get" {
		if len(positionals) != 1 || !validDesktopIdentifier(positionals[0]) {
			return invocation, errors.New("provide one task or file ID after the command")
		}
		if needsTaskID {
			invocation.command.Payload["taskId"] = positionals[0]
		} else {
			invocation.command.Payload["id"] = positionals[0]
		}
	} else if len(positionals) != 0 {
		return invocation, errors.New("unexpected argument; see glowbom desktop --help")
	}
	invocation.command.Operation = operation
	invocation.command.Actor = values["agent"]
	if operation == "tasks.create" {
		invocation.command.Actor = ""
	}
	if actor := values["actor"]; actor != "" {
		invocation.command.Actor = actor
	}
	payload := invocation.command.Payload
	if revision := values["revision"]; revision != "" {
		number, err := strconv.Atoi(revision)
		if err != nil || number < 1 {
			return invocation, errors.New("--revision must be a positive whole number")
		}
		payload["revision"] = number
	}
	if attempt := values["attempt"]; attempt != "" {
		if !validDesktopIdentifier(attempt) {
			return invocation, errors.New("--attempt must be the attempt ID returned by next")
		}
		payload["attemptId"] = attempt
	}
	switch operation {
	case "files.get":
		if values["output"] == "" {
			return invocation, errors.New("files get requires --output with a new file path")
		}
		invocation.command.Operation = "attachments.read"
		invocation.output = values["output"]
	case "agents.register":
		if values["username"] == "" || values["name"] == "" {
			return invocation, errors.New("register requires --username and --name")
		}
		copyDesktopValues(payload, values, "username", "name", "provider")
	case "agents.whoami":
		if values["agent"] == "" {
			return invocation, errors.New("whoami requires --agent")
		}
		payload["username"] = values["agent"]
	case "tasks.create":
		if values["agent"] == "" || values["title"] == "" {
			return invocation, errors.New("create requires --agent and --title")
		}
		instructions, err := desktopTextOption(values, "instructions", "instructions-file")
		if err != nil {
			return invocation, err
		}
		if strings.TrimSpace(instructions) == "" {
			return invocation, errors.New("create requires --instructions or --instructions-file")
		}
		copyDesktopValues(payload, values, "agent", "title", "project")
		payload["instructions"] = instructions
	case "tasks.next":
		if values["agent"] == "" {
			return invocation, errors.New("next requires --agent")
		}
		payload["agent"] = values["agent"]
	case "tasks.list":
		copyDesktopValues(payload, values, "agent")
	case "tasks.reply", "tasks.submit":
		message, err := desktopTextOption(values, "message", "message-file")
		if err != nil {
			return invocation, err
		}
		if strings.TrimSpace(message) == "" {
			return invocation, errors.New("provide --message or --message-file")
		}
		payload["text"] = message
		if operation == "tasks.submit" && (values["attempt"] == "" || values["revision"] == "") {
			return invocation, errors.New("submit requires --attempt and --revision from next")
		}
	case "tasks.accept":
		if !validDesktopIdentifier(values["result"]) {
			return invocation, errors.New("accept requires --result with the result ID to accept")
		}
		payload["resultId"] = values["result"]
	}
	if len(files) > 0 {
		attachments, err := readDesktopAttachments(files)
		if err != nil {
			return invocation, err
		}
		payload["attachments"] = attachments
	}
	requestID := values["request-id"]
	if requestID != "" && !validDesktopIdentifier(requestID) {
		return invocation, errors.New("--request-id must use 1 to 128 letters, numbers, underscores, or hyphens")
	}
	mutating := operation == "agents.register" || operation == "tasks.create" || operation == "tasks.next" ||
		operation == "tasks.reply" || operation == "tasks.submit" || operation == "tasks.accept"
	if mutating && requestID == "" {
		var err error
		requestID, err = randomHexSecret(16)
		if err != nil {
			return invocation, errors.New("could not create a request ID")
		}
	}
	invocation.command.RequestID = requestID
	return invocation, nil
}

func copyDesktopValues(payload map[string]any, values map[string]string, names ...string) {
	for _, name := range names {
		if value := values[name]; value != "" {
			payload[name] = value
		}
	}
}

func desktopTextOption(values map[string]string, textFlag, fileFlag string) (string, error) {
	text, path := values[textFlag], values[fileFlag]
	if text != "" && path != "" {
		return "", fmt.Errorf("use either --%s or --%s, not both", textFlag, fileFlag)
	}
	if path != "" {
		return readDesktopText(path)
	}
	if len(text) > desktopTextLimit || strings.ContainsRune(text, '\x00') {
		return "", fmt.Errorf("--%s must be text of at most 128 KiB", textFlag)
	}
	return text, nil
}
