import UIKit
import Flutter
import RealityKit

@UIApplicationMain
@objc class AppDelegate: FlutterAppDelegate {
  // Keep a reference to the ARView or the custom UIView you've created
  var arViewContainer: GlowbyARView?

  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    
    GeneratedPluginRegistrant.register(with: self)
    
    let controller : FlutterViewController = window?.rootViewController as! FlutterViewController
    let arChannel = FlutterMethodChannel(name: "com.glowbom/ar", binaryMessenger: controller.binaryMessenger)
    
    arChannel.setMethodCallHandler({
      [weak self] (call: FlutterMethodCall, result: @escaping FlutterResult) -> Void in
      // Identify which method is being called from Flutter
      if call.method == "loadARView" {
        self?.loadARView(result: result)
      } else {
        result(FlutterMethodNotImplemented)
      }
    })

    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

    private func loadARView(result: @escaping FlutterResult) {
        // Ensure you are running on the main thread
        DispatchQueue.main.async {
            // If the AR view is not already initialized
            if self.arViewContainer == nil {
                // Initialize the AR view
                self.arViewContainer = GlowbyARView(frame: self.window?.bounds ?? .zero)
                // Add the AR view to the view hierarchy
                self.window?.rootViewController?.view.addSubview(self.arViewContainer!)
            }
            // Return success to the Flutter side
            result(nil)
        }
    }
}
