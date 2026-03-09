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
