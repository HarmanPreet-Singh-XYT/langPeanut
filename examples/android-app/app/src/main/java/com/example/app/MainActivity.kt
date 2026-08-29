package com.example.app

import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun OrderScreen() {
    Text(text = "Welcome back, {name}!")
    Button(onClick = { /* process order */ }) {
        Text(text = "Submit Order")
    }
}
