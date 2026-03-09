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
