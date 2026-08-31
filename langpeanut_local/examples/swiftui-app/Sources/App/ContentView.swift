import SwiftUI

public struct ContentView: View {
    @State private var notificationsEnabled = true

    public init() {}

    public var body: some View {
        NavigationStack {
            VStack(spacing: 20) {
                Text("Welcome back, {name}!")
                    .font(.headline)
                
                Button("Submit Order") {
                    print("Order clicked")
                }
                .buttonStyle(.borderedProminent)
            }
            .navigationTitle("Dashboard")
        }
    }
}
