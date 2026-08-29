package com.example.app

import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource

@Composable
fun OrderScreen() {
    Text(text = stringResource(R.string.mainactivityWelcomeBack))
    Button(onClick = { /* process order */ }) {
        Text(text = stringResource(R.string.mainactivitySubmitOrder))
    }
}
