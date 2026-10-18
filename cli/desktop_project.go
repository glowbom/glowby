package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

func isDesktopProjectCommand(group string) bool {
	switch group {
	case "project", "models", "chat", "build", "runs":
		return true
	}
	return false
}

func parseDesktopProjectCommand(operation string, args []string) (desktopInvocation, error) {
	invocation := desktopInvocation{command: desktopCommand{Version: 1, Operation: operation, Payload: map[string]any{}}}
	allowed := map[string]string{
		"project.current": "",
		"models.list":     "",
		"chat.send":       "project context model actor request-id message message-file",
		"build.start":     "project context model actor request-id instructions instructions-file",
		"runs.show":       "",
	}
	flagList, ok := allowed[operation]
	if !ok {
		return invocation, errors.New("unknown Desktop command; see glowbom desktop --help")
	}
	flags := map[string]bool{}
	for _, name := range strings.Fields(flagList + " json") {
		flags[name] = true
	}
	values := map[string]string{}
	var positionals []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "--") {
			positionals = append(positionals, arg)
			continue
		}
		name, value, supplied := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		if !flags[name] {
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
		if strings.TrimSpace(value) == "" || strings.ContainsRune(value, '\x00') {
			return invocation, fmt.Errorf("--%s requires a nonempty value without null characters", name)
		}
		if _, exists := values[name]; exists {
			return invocation, fmt.Errorf("--%s may only be given once", name)
		}
		values[name] = value
	}
	if operation == "runs.show" {
		if len(positionals) != 1 || !validDesktopIdentifier(positionals[0]) {
			return invocation, errors.New("provide one run ID after runs show")
		}
		invocation.command.Payload["id"] = positionals[0]
	} else if len(positionals) != 0 {
		return invocation, errors.New("unexpected argument; see glowbom desktop --help")
	}
	mutating := operation == "chat.send" || operation == "build.start"
	if !mutating {
		return invocation, nil
	}
	if project := values["project"]; project != "" && !filepath.IsAbs(project) {
		return invocation, errors.New("--project must be an absolute project path")
	}
	if value := values["context"]; value != "" {
		if len(value) > 128 || !utf8.ValidString(value) || strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return invocation, errors.New("--context must use up to 128 bytes without control characters")
		}
		invocation.command.Payload["contextId"] = value
	}
	textFlag := "message"
	if operation == "build.start" {
		textFlag = "instructions"
	}
	text, err := desktopTextOption(values, textFlag, textFlag+"-file")
	if err != nil {
		return invocation, err
	}
	if strings.TrimSpace(text) == "" {
		return invocation, fmt.Errorf("%s requires --%s or --%s-file", strings.ReplaceAll(operation, ".", " "), textFlag, textFlag)
	}
	copyDesktopValues(invocation.command.Payload, values, "project", "model")
	invocation.command.Payload[textFlag] = text
	invocation.command.Actor = values["actor"]
	requestID := values["request-id"]
	if requestID != "" && !validDesktopIdentifier(requestID) {
		return invocation, errors.New("--request-id must use 1 to 128 letters, numbers, underscores, or hyphens")
	}
	if requestID == "" {
		requestID, err = randomHexSecret(16)
		if err != nil {
			return invocation, errors.New("could not create a request ID")
		}
	}
	invocation.command.RequestID = requestID
	return invocation, nil
}
