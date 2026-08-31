// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "SwiftUIExampleApp",
    platforms: [.iOS(.v17), .macOS(.v14)],
    products: [
        .library(name: "SwiftUIExampleApp", targets: ["SwiftUIExampleApp"]),
    ],
    targets: [
        .target(name: "SwiftUIExampleApp"),
    ]
)
