package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", glowbyBackendHomeHandler)
	http.HandleFunc("/favicon.png", glowbyFaviconHandler)
	http.HandleFunc("/logo-svg.svg", glowbyLogoSVGHandler)

	http.HandleFunc("/chatWithAI", chatWithAIHandler)
	http.HandleFunc("/webSearch", webSearchHandler)
	http.HandleFunc("/analyzeVideo", analyzeVideoHandler)

	http.HandleFunc("/image", generateImageHandler)
	http.HandleFunc("/audio", generateAudioHandler)
	http.HandleFunc("/audio/voices", listElevenLabsVoicesHandler)

	// Veo video generation endpoints
	http.HandleFunc("/generateVeoVideo", generateVeoVideoHandler)
	http.HandleFunc("/pollVeoOperation", pollVeoOperationHandler)

	// OpenCode agent endpoints
	http.HandleFunc("/opencode/translate", openCodeTranslateHandler)
	http.HandleFunc("/opencode/health", openCodeHealthHandler)
	http.HandleFunc("/opencode/about", openCodeAboutHandler)
	http.HandleFunc("/opencode/about/project-description", openCodeProjectDescriptionHandler)
	http.HandleFunc("/opencode/auth/status", openCodeAuthStatusHandler)
	http.HandleFunc("/opencode/auth/openai/oauth/start", openCodeOpenAIOAuthStartHandler)
	http.HandleFunc("/opencode/auth/openai/oauth/status", openCodeOpenAIOAuthStatusHandler)
	http.HandleFunc("/opencode/auth/openai/oauth/callback", openCodeOpenAIOAuthCallbackHandler)
	http.HandleFunc("/opencode/auth/openai/connect", openCodeOpenAIConnectHandler)
	http.HandleFunc("/opencode/auth/openai/disconnect", openCodeOpenAIDisconnectHandler)
	http.HandleFunc("/opencode/project/init", openCodeInitProjectHandler)
	http.HandleFunc("/opencode/project", openCodeGetProjectHandler)
	http.HandleFunc("/opencode/project/pick", openCodePickProjectFolderHandler)
	http.HandleFunc("/opencode/instructions/files/pick", openCodePickInstructionFilesHandler)
	http.HandleFunc("/opencode/project/ide/status", openCodeProjectIDEStatusHandler)
	http.HandleFunc("/opencode/project/open", openCodeProjectOpenHandler)
	http.HandleFunc("/opencode/models/available", openCodeAvailableModelsHandler)
	http.HandleFunc("/providers/openai/models", openAIModelsHandler)
	http.HandleFunc("/opencode/refine", openCodeRefineHandler)
	http.HandleFunc("/opencode/media/postpass", openCodeMediaPostPassHandler)
	http.HandleFunc("/opencode/verify", openCodeVerifyHandler)
	http.HandleFunc("/opencode/question/respond", openCodeQuestionRespondHandler)
	http.HandleFunc("/opencode/permission/respond", openCodePermissionRespondHandler)

	port := os.Getenv("GLOWBOM_PORT")
	if port == "" {
		port = "4569"
	}
	fmt.Printf("Server running on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
