# Glowbom AI Extensions Guide for ChatGPT

This guide is intended for ChatGPT with Code Interpreter to understand the context of a given project and to provide assistance in generating code for AI extensions.

This repository contains a basic structure for a Custom application along with a special `AiExtensions.kt` file. This file is designated for OpenAI model to add or modify code, providing optional features or enhancements to the existing Kotlin Jetpack Compose project.

## Understanding the Code

- `MainActivity.kt`: This is the main file for the Custom application. It represents a simple one screen app.
- `AiExtensions.kt`: This is where you can add custom AI extension code. The `GlowbyScreen` class is an example of what can be added here.

## Interactive Features

For tasks that involve calculations or data manipulation, the generated code should include interactive features that allow users to input data and see the results reflected in the UI. The results should be presented in a clear and user-friendly manner.

For example, if the task is to implement a function that performs calculations based on user input, the generated code should include:

1. Input fields for the user to provide the necessary data.
2. A way for the user to trigger the calculation. This could be a button that the user can press to run the function once the necessary data has been inputted.
3. A way to display the results of the calculation in the UI.

The AI extension should handle any necessary error checking and data validation to ensure that the user-provided data is suitable for the calculation. If the data is not suitable, the AI extension should provide clear feedback to the user about what needs to be corrected.

## Additional Guidelines

1. **Problem Statement Understanding**: Understand the problem statement thoroughly before proceeding with the solution. Ask for clarifications if the problem statement isn't clear.
2. **Solution Language**: Confirm the language of the solution. The solution for the current project is expected to be in Kotlin.
3. **LeetCode Submissions**: If the problem is from a competitive programming platform like LeetCode, provide the solution suitable for that platform, considering the given method signature and expected return type.
4. **Code Testing**: Test the generated code for any syntax or logical errors before delivering the solution. Also, consider different edge cases to ensure the solution is robust.
5. **Performance Considerations**: Provide solutions that are not just correct but also efficient.

## Specific Notes for Kotlin Jetpack Compose

1. **UI Components**: When adding or modifying UI components, use Material Design 3 components. This is the design system currently in use in the project.
2. **Function Invocation**: If you add a new composable function inside `AiExtensions.kt`, make sure it's invoked correctly inside `MainActivity.kt`. Ensure that the function is accessible and that the call follows the correct syntax.
3. **Typography**: When setting typography, refer to the available typography in `MaterialTheme.typography` for the project. Not all typography styles may be available.

## Generating Code for AI Extensions

The process of creating an AI extension involves generating Kotlin code that extends or enhances the existing Custom application. 

Here are the steps to follow:

1. Glowbom users will attach the `ai.zip` file along with a description of what they want to create (e.g., "a screen with a list of 3 default habits in a calendar format - workout, meditate, drink water").
2. Based on this description, generate Kotlin code within the `AiExtensions` object in the `AiExtensions.kt` file that fits the users' needs and is compatible with the existing codebase.
3. Test the generated code for any syntax or logical errors before delivering the solution. Also, consider different edge cases to ensure the solution is robust.
4. Once the code has been tested and confirmed to be working as expected, return the entire updated `AiExtensions.kt` file content. This is the final output of the task.
5. Return the Entire File: After generating and testing the code, always return the entire updated `AiExtensions.kt` file content. Do not return only a part of the code or a class name. The entire content of the file is necessary for the user to easily integrate the new code with their existing codebase.

Remember, any code added should be clearly marked with comments for users to understand what has been added or changed.

Happy coding!

Note: After generating and testing the code, always return the entire updated `AiExtensions.kt` file content. This is the final output of the task.

Here's the code:

AiExtensions.kt:

package com.glowbom.custom

import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.wrapContentSize
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp

class AiExtensions {
companion object {
// Flag to enable or disable the GlowbyScreen
var enabled = false

        // Title for the GlowbyScreen
        var title = "App"

        /**
         * Composable function that displays the GlowbyScreen when enabled is set to true.
         * The GlowbyScreen consists of an image "glowby" which is displayed in the center of the screen.
         *
         * @param modifier Modifier to be applied to the Box layout
         */
        @Composable
        fun GlowbyScreen(
            modifier: Modifier = Modifier
        ) {
            if (enabled) {
                Box(
                    modifier = modifier
                        .fillMaxSize()
                ) {
                    Image(
                        painter = painterResource(id = R.drawable.glowbom),
                        contentDescription = "Glowby Screen",
                        contentScale = ContentScale.FillBounds,
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(300.dp)
                            .wrapContentSize(Alignment.Center)
                    )
                }
            }
        }
    }
}

MainActivity.kt:

package com.glowbom.custom

import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.graphics.drawable.Drawable
import android.net.Uri
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import com.bumptech.glide.Glide
import com.bumptech.glide.request.target.CustomTarget
import com.glowbom.custom.ui.theme.CustomTheme
import com.google.gson.Gson
import com.google.gson.reflect.TypeToken
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class CustomState(val context: Context) {
val appScreen = mutableStateOf("Loading")
lateinit var content: Map<String, Any>

    val title: String
        get() = content["title"] as String? ?: "Custom App"
    var mainColor = mutableStateOf(Color.Blue)
    val questions: List<Map<String, Any>>
        get() {
            val questionsAny = content["questions"]
            return if (questionsAny is List<*>) {
                questionsAny.filterIsInstance<Map<String, Any>>()
            } else {
                emptyList()
            }
        }
    init {
        CoroutineScope(Dispatchers.IO).launch {
            content = loadContentFromAssets()
            withContext(Dispatchers.Main) {
                mainColor.value = getColorFromString(content["main_color"] as String? ?: "Blue")
                appScreen.value = "Questions"
            }
        }
    }

    private suspend fun loadContentFromAssets(): Map<String, Any> {
        val data = context.assets.open("custom.glowbom").bufferedReader().use { it.readText() }
        return Gson().fromJson(data, object : TypeToken<Map<String, Any>>() {}.type)
    }
}

fun getColorFromString(colorString: String): Color {
return when (colorString.lowercase()) {
"black" -> Color.Black
"blue" -> Color.Blue
"green" -> Color.Green
"grey" -> Color.Gray
"red" -> Color.Red
else -> Color.Blue
}
}

@Composable
fun LoadingScreen() {
Box(
contentAlignment = Alignment.Center,
modifier = Modifier.fillMaxSize()
) {
Text(text = "Loading...")
}
}

@Composable
fun GlowbomScreen() {
Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
Image(
painter = painterResource(id = R.drawable.glowbom),
contentDescription = "Glowbom Logo"
)
}
}

@Composable
fun ElementsScreen(elements: List<Map<String, Any>>) {
Box(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
Column(
modifier = Modifier.fillMaxWidth(),
horizontalAlignment = Alignment.CenterHorizontally
) {
elements.forEach { element ->
when (val description = element["description"] as String?) {
"Image" -> ImageElement(element)
"Text" -> TextElement(element)
"Button" -> ButtonElement(element)
"Input" -> InputElement(element)
else -> Text(text = "Unsupported element type: $description")
}
}
}
}
}

@Composable
fun ImageElement(element: Map<String, Any>) {
// Get the image URL from the element
val imageUrl = (element["buttonsTexts"] as? List<*>)?.first() as? String ?: ""

    val imageBitmap = rememberImage(imageUrl)

    Box(Modifier.padding(16.dp)) {
        imageBitmap?.let {
            Image(
                bitmap = it,
                contentDescription = null, // descriptive text for the visually impaired
                contentScale = ContentScale.Crop,
                modifier = Modifier
                    .height(200.dp)
                    .width(320.dp)
            )
        }
    }
}

@Composable
fun rememberImage(url: String): ImageBitmap? {
var image by remember { mutableStateOf<ImageBitmap?>(null) }
val context = LocalContext.current

    Glide.with(context)
        .asBitmap()
        .load(url)
        .into(object : CustomTarget<Bitmap>() {
            override fun onResourceReady(
                resource: Bitmap,
                transition: com.bumptech.glide.request.transition.Transition<in Bitmap>?
            ) {
                image = resource.asImageBitmap()
            }

            override fun onLoadCleared(placeholder: Drawable?) {
                // Handle the case when the image loading is cancelled or failed.
            }
        })

    return image
}


@Composable
fun TextElement(element: Map<String, Any>) {
val text = when (val buttonsTexts = element["buttonsTexts"]) {
is String -> buttonsTexts
is List<*> -> buttonsTexts.joinToString("\n") // Combine all texts with newline
else -> ""
}

    Text(
        text = text,
        color = Color.Black,
        modifier = Modifier
            .padding(16.dp)
            .width(320.dp)
    )
}


@Composable
fun ButtonElement(element: Map<String, Any>) {
// Get the button title and URL from the element
val buttonsTexts = element["buttonsTexts"] as? List<*>
val buttonTitle = buttonsTexts?.getOrNull(0) as? String ?: ""
val urlString = buttonsTexts?.getOrNull(1) as? String
val context = LocalContext.current

    Button(
        onClick = { urlString?.let { openUrl(it, context) } },
        modifier = Modifier
            .padding(16.dp)
            .width(320.dp)
    ) {
        Text(text = buttonTitle)
    }
}

fun openUrl(url: String, context: Context) {
val intent = Intent(Intent.ACTION_VIEW)
intent.data = Uri.parse(url)
context.startActivity(intent)
}


@Composable
fun InputElement(element: Map<String, Any>) {
// TODO: Implement InputElement with the proper data from the element
}



@Composable
fun CustomScreen(customState: CustomState) {
when {
customState.appScreen.value == "Loading" -> LoadingScreen()
customState.appScreen.value == "Glowbom" -> GlowbomScreen()
AiExtensions.enabled -> AiExtensions.GlowbyScreen()
customState.appScreen.value == "Questions" -> ElementsScreen(customState.questions)
else -> Text(text = "Loaded, but appScreen is '${customState.appScreen.value}'")
}
}


class MainActivity : ComponentActivity() {
private val customState by lazy { CustomState(this) }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            CustomTheme {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MaterialTheme.colorScheme.background
                ) {
                    CustomScreen(customState)
                }
            }
        }
    }
}
