# Workspace Instructions

## Commit Messages
- After making code changes, always suggest a one-line conventional commit message.
- Prefer the format `type: summary`.

## Bundled Project Template

When working in `project/`, use `project/prototype/index.html` as the reference design.

### Primary Goal

All destination projects in `project/` must look and function exactly like the prototype. This includes:
- Identical visual appearance: colors, fonts, spacing, and layout
- Same functionality and user interactions
- All prototype images included and displayed correctly
- App icon matching `project/icon.png`

### App Metadata

- Display Name: `Glowby`
- Bundle ID: `app.glowbom.glowby`
- Version: `1.0`
- Build Number: `1`

Update these values in each platform output:

#### iOS
- Display Name: Xcode target -> General -> Display Name, or `CFBundleDisplayName` in `Info.plist`
- Bundle ID: Xcode target -> Signing & Capabilities -> Bundle Identifier
- Version: Xcode target -> General -> Version, or `CFBundleShortVersionString` in `Info.plist`
- Build: Xcode target -> General -> Build, or `CFBundleVersion` in `Info.plist`

#### Android
- Display Name: `project/android/app/src/main/res/values/strings.xml` -> `app_name`
- Bundle ID: `project/android/app/build.gradle` -> `applicationId`
- Version: `project/android/app/build.gradle` -> `versionName`
- Build: `project/android/app/build.gradle` -> `versionCode`

#### Games
- Display Name: update in the engine project settings
- Bundle ID: update in the engine export settings
- Version: update in the engine project/export settings

### Default Tasks

#### 0. Production Readiness
- Review the prototype and ensure all UI elements are properly implemented
- Verify colors, typography, and spacing match the prototype
- Check that user interactions work as expected
- Full screen layout: the app or game should fill the screen with no unnecessary empty white space
- Sprite transparency for games: sprites must use transparent backgrounds

#### 1. Copy Images from Prototype
- Copy all media assets from `project/prototype/assets/` to platform-specific asset directories
- Reference `project/prototype/assets.json` for asset metadata and original prompts
- Ensure image and video formats are compatible with each platform

#### 2. Copy and Resize App Icon
- Source icon: `project/icon.png`
- Resize and place icons in the correct locations for each platform

#### Apple (SwiftUI)
- Single SwiftUI project under `project/apple/` targets iOS, iPadOS, macOS, visionOS, watchOS, and tvOS
- Update the SwiftUI extension file such as `project/apple/Custom/AiExtensions.swift`
- Copy images into the asset catalog under `project/apple/Custom/Assets.xcassets/`
- Update `Contents.json` in the AppIcon asset with the correct filenames

#### Android (Jetpack Compose)
- Update the Android extension file such as `AiExtensions.kt`
- Copy images into Android drawable resources
- Name icon files `ic_launcher.png` and `ic_launcher_round.png`

#### Web (Next.js)
- Place the app icon in `project/web/src/app/icon.png`
- Also copy the icon to `project/web/public/icon.png` for static references
- Keep `project/web/src/app/favicon.ico` consistent if it exists
- Ensure web metadata and title reflect the app identity

#### 3. Verify No Compile Errors
- Build each platform project and fix any compilation errors
- For Apple: open the destination project in Xcode and build
- For Android: open the destination in Android Studio and sync/build
- For games: open the project and run the main scene

### Target Destinations

iOS (SwiftUI), Android (Jetpack Compose), Web (Next.js)

### Reference Files

- Prototype HTML: `project/prototype/index.html`
- Assets: `project/prototype/assets/`
- Asset metadata: `project/prototype/assets.json`
- App Icon: `project/icon.png`
- Project Manifest: `project/glowbom.json`

### Notes

- If the user gives different instructions, follow those instead of the defaults above
- Keep the prototype as the source of truth for visual design
- Test on actual devices when possible
- For games, ensure sprite assets keep transparent backgrounds
