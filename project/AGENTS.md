# Agent Instructions

This project was exported from Glowbom. Use prototype/index.html as the reference design.

## Primary Goal

**All destination projects must look and function exactly like the prototype.** This includes:
- Identical visual appearance (colors, fonts, spacing, layout)
- Same functionality and user interactions
- All images from the prototype included and displayed correctly
- App icon matching the provided icon.png

## App Metadata

- **Display Name**: `Glowby`
- **Bundle ID**: `app.glowbom.glowby`
- **Version**: `1.0`
- **Build Number**: `1`

Update these values in each platform:

### iOS
- **Display Name**: In Xcode, select target → General → Display Name, or edit `Info.plist` key `CFBundleDisplayName`
- **Bundle ID**: In Xcode, select target → Signing & Capabilities → Bundle Identifier
- **Version**: In Xcode, select target → General → Version (also `CFBundleShortVersionString` in Info.plist)
- **Build**: In Xcode, select target → General → Build (also `CFBundleVersion` in Info.plist)

### Android
- **Display Name**: In `android/app/src/main/res/values/strings.xml`, update `app_name`
- **Bundle ID**: In `android/app/build.gradle`, update `applicationId` under `defaultConfig`
- **Version**: In `android/app/build.gradle`, update `versionName` under `defaultConfig`
- **Build**: In `android/app/build.gradle`, update `versionCode` under `defaultConfig` (integer)

### Games
- **Display Name**: Update in the game engine project settings (e.g., Godot `project.godot`, Unity/Unreal project settings)
- **Bundle ID**: Update the package/bundle identifier in the engine’s export settings
- **Version**: Update the version field in the engine’s project/export settings

## Default Tasks

Complete the following tasks to make this project production-ready:

### 0. Production Readiness
- Review the prototype and ensure all UI elements are properly implemented
- Verify color schemes, typography, and spacing match the prototype
- Check that all user interactions work as expected
- **Full screen layout**: App or game must take the entire screen with no empty white space (no letterboxing). Especially for games, content should fill the display; only leave empty space if the user explicitly asks. Use safe area insets properly but ensure content fills the display.
- **Sprite transparency (games)**: Sprites must have transparent backgrounds (PNG with alpha). Remove any solid/boxed backgrounds so sprites never appear as squares.

### 1. Copy Images from Prototype
- Copy all media assets from `prototype/assets/` to platform-specific asset directories
- Reference `prototype/assets.json` for asset metadata and original prompts
- Ensure image/video formats are compatible with each platform

### 2. Copy and Resize App Icon
- Source icon: `icon.png` in project root (if available)
- Resize and place icons in the correct locations for each platform:

### Apple (SwiftUI)
- Single SwiftUI project under `apple/` targets iOS, iPadOS, macOS, visionOS, watchOS, and tvOS
- Update the SwiftUI extension file (e.g. `apple/Custom/AiExtensions.swift`) with the UI code
- Copy images to the asset catalog (e.g. `apple/Custom/Assets.xcassets/`)
- App icons go in the AppIcon set:
  - iPhone Notification: 20pt @2x (40px), @3x (60px)
  - iPhone Settings: 29pt @2x (58px), @3x (87px)
  - iPhone Spotlight: 40pt @2x (80px), @3x (120px)
  - iPhone App: 60pt @2x (120px), @3x (180px)
  - App Store: 1024pt @1x (1024px)
- Update `Contents.json` in AppIcon.appiconset with correct filenames
### Android (Jetpack Compose)
- Update the Android extension file (e.g. `*/AiExtensions.kt`)
- Copy images to the platform drawable resources
- App icons go in mipmap directories:
  - `mipmap-mdpi/`: 48x48px
  - `mipmap-hdpi/`: 72x72px
  - `mipmap-xhdpi/`: 96x96px
  - `mipmap-xxhdpi/`: 144x144px
  - `mipmap-xxxhdpi/`: 192x192px
- Name icon files `ic_launcher.png` and `ic_launcher_round.png`
### Web (Next.js)
- Place the app icon in `web/src/app/icon.png` (preferred, 1024x1024 square)
- Also copy icon to `web/public/icon.png` for static references
- If `web/src/app/favicon.ico` exists, keep it consistent with the generated icon
- Ensure web metadata/title in `web/src/app/layout.tsx` reflects the app identity
- Verify icon is visible in browser tab and install surfaces

### 3. Verify No Compile Errors
- Build each platform project and fix any compilation errors
- For Apple: Open the destination Xcode project in Xcode and build (Cmd+B)
- For Android: Open the destination in Android Studio and sync/build
- For Games: Open the game project and run the main scene

## Target Destinations

iOS (SwiftUI), Android (Jetpack Compose), Web (Next.js)

## Reference Files

- **Prototype HTML**: `prototype/index.html`
- **Assets**: `prototype/assets/`
- **Asset metadata**: `prototype/assets.json` (contains prompts and file mappings)
- **App Icon**: `icon.png` (root directory)
- **Project Manifest**: `glowbom.json` (contains project name, bundleID, and target destinations)

## Notes

- If user provides different instructions, follow those instead of defaults above
- Keep the prototype as the source of truth for visual design
- Test on actual devices when possible
- **For games**: Ensure all game sprites have transparent backgrounds (PNG with alpha). Game elements should not appear as squares or rectangles - remove any solid background colors from sprite images.