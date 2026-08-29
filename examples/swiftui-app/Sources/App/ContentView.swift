import SwiftUI

public struct ContentView: View {
    @State private var notificationsEnabled = true

    public init() {}

    public var body: some View {
        NavigationStack {
            VStack(spacing: 20) {
                Text("contentviewWelcomeback")
                    .font(.headline)
                
                Button("contentviewSubmitorder") {
                    print("Order clicked")
                }
                .buttonStyle(.borderedProminent)
            }
            .navigationTitle("contentviewDashboard")
        }
    }
}
