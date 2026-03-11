package com.glowbom.custom

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.rounded.Face
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

class AiExtensions {
    companion object {
        var enabled = true
        var title = "App"

        @Composable
        fun GlowbyScreen(
            modifier: Modifier = Modifier
        ) {
            if (enabled) {
                val page = Color(0xFFF5F5F5)
                val ink = Color(0xFF111111)
                val accent = Color(0xFF29DE92)

                Box(
                    modifier = modifier
                        .fillMaxSize()
                        .background(page),
                    contentAlignment = Alignment.Center
                ) {
                    BoxWithConstraints(
                        modifier = Modifier
                            .fillMaxWidth()
                            .widthIn(max = 360.dp),
                        contentAlignment = Alignment.Center
                    ) {
                        Column(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.Center
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(96.dp)
                                    .background(Color.White, CircleShape),
                                contentAlignment = Alignment.Center
                            ) {
                                Box(
                                    modifier = Modifier
                                        .size(64.dp)
                                        .background(accent.copy(alpha = 0.18f), CircleShape),
                                    contentAlignment = Alignment.Center
                                ) {
                                    Icon(
                                        imageVector = Icons.Rounded.Face,
                                        contentDescription = "Character icon",
                                        tint = accent,
                                        modifier = Modifier.size(34.dp)
                                    )
                                }
                            }

                            Text(
                                text = "Build anything.",
                                color = ink,
                                fontSize = 40.sp,
                                fontWeight = FontWeight.ExtraBold,
                                textAlign = TextAlign.Center,
                                lineHeight = 44.sp,
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .widthIn(max = 320.dp)
                            )

                            Text(
                                text = buildAnnotatedString {
                                    append("Made with ")
                                    pushStyle(SpanStyle(color = accent, fontWeight = FontWeight.SemiBold))
                                    append("Glowbom")
                                    pop()
                                },
                                color = ink.copy(alpha = 0.55f),
                                fontSize = 14.sp,
                                fontWeight = FontWeight.Medium,
                                textAlign = TextAlign.Center,
                                modifier = Modifier.fillMaxWidth()
                            )
                        }
                    }
                }
            }
        }
    }
}