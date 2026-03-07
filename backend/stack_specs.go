package main

// Stack-specific instruction templates shared across translation prompts.

var baseStackInstructions = map[string]string{
	"swiftui": `Create a production-ready SwiftUI iOS app:
- Proper Xcode project structure (Package.swift or .xcodeproj)
- MVVM architecture with ObservableObject ViewModels
- Error handling with proper Swift error types
- Async/await for any data operations
- Run 'swift build' to verify compilation
- Fix any compiler errors before completing`,

	"kotlin": `Create a production-ready Jetpack Compose Android app:
- Proper Gradle project structure (build.gradle.kts)
- MVVM architecture with ViewModel and StateFlow
- Proper error handling with sealed classes
- Coroutines for async operations
- Run './gradlew build' to verify compilation
- Fix any compiler errors before completing`,

	"nextjs": `Create a production-ready Next.js 16+ web app:
- App Router structure (app/ directory)
- TypeScript with strict mode
- Prefer Server Components; use Client Components only when interactivity is required
- Use next/image and next/font for performance
- Add proper loading, error, and not-found states where applicable
- Run 'npm run lint && npm run build' to verify
- Fix any TypeScript/lint/build errors before completing`,

	"godot": `Create a production-ready Godot 4 game project:
- Proper scene structure (.tscn files)
- GDScript with type hints
- Signal-based architecture
- Resource preloading
- Verify scenes load without errors`,
}

var projectStackInstructions = map[string]string{
	"swiftui": `Create a production-ready SwiftUI iOS app:
- Navigate to ios/ directory and create all files there
- Proper Xcode project structure (Package.swift or .xcodeproj)
- MVVM architecture with ObservableObject ViewModels
- Error handling with proper Swift error types
- Async/await for any data operations
- Copy any assets from prototype/assets/ to the appropriate location
- Report progress: "Starting SwiftUI app creation"
- Run 'swift build' to verify compilation and report results
- Fix any compiler errors before completing, report each fix`,

	"kotlin": `Create a production-ready Jetpack Compose Android app:
- Navigate to android/ directory and create all files there
- Proper Gradle project structure (build.gradle.kts)
- MVVM architecture with ViewModel and StateFlow
- Proper error handling with sealed classes
- Coroutines for async operations
- Copy any assets from prototype/assets/ to app/src/main/res/
- Report progress: "Starting Android app creation"
- Run './gradlew build' to verify compilation and report results
- Fix any compiler errors before completing, report each fix`,

	"nextjs": `Create a production-ready Next.js 16+ web app:
- Navigate to web/ directory and create all files there
- App Router structure (src/app/ directory)
- TypeScript with strict mode
- Prefer Server Components; use Client Components only when interactivity is required
- Proper loading, error, and not-found states
- Copy any assets from prototype/assets/ to public/assets/
- Report progress: "Starting Next.js app creation"
- Run 'npm install && npm run lint && npm run build' to verify and report results
- Fix any TypeScript/lint/build errors before completing, report each fix`,

	"godot": `Create a production-ready Godot 4 game project:
- Navigate to godot/ directory and create all files there
- Proper scene structure (.tscn files)
- GDScript with type hints
- Signal-based architecture
- Resource preloading
- Import any assets from prototype/assets/
- Report progress: "Starting Godot project creation"
- Verify scenes load without errors and report results
- Fix any issues before completing, report each fix`,
}

func stackInstructionsFor(targetLang string, projectMode bool) string {
	normalized := targetLang
	switch targetLang {
	case "ios":
		normalized = "swiftui"
	case "android":
		normalized = "kotlin"
	case "web":
		normalized = "nextjs"
	case "games":
		normalized = "godot"
	}

	if projectMode {
		if instructions, ok := projectStackInstructions[normalized]; ok && instructions != "" {
			return instructions
		}
	} else if instructions, ok := baseStackInstructions[normalized]; ok && instructions != "" {
		return instructions
	}
	return "Create production-ready code with proper project structure and error handling."
}
