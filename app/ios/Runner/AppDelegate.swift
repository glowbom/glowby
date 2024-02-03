import UIKit
import Flutter
import RealityKit

@UIApplicationMain
@objc class AppDelegate: FlutterAppDelegate {
  var arViewContainer: GlowbyARView?

  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {

      GeneratedPluginRegistrant.register(with: self)
      print("howdy")
    
    guard let controller = window?.rootViewController as? FlutterViewController else {
      print("RootViewController is not of type FlutterViewController")
      return false
    }
    let arChannel = FlutterMethodChannel(name: "com.glowbom/ar", binaryMessenger: controller.binaryMessenger)
    
    arChannel.setMethodCallHandler({
      [weak self] (call: FlutterMethodCall, result: @escaping FlutterResult) -> Void in
      print("Received method call: \(call.method)")
      if call.method == "loadARView" {
        self?.loadARView(result: result)
      } else {
        result(FlutterMethodNotImplemented)
      }
    })

    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  private func loadARView(result: @escaping FlutterResult) {
    DispatchQueue.main.async {
      if self.arViewContainer == nil {
        print("ARViewContainer is nil, initializing ARView...")
        self.arViewContainer = GlowbyARView(frame: self.window?.bounds ?? .zero)
        guard let arViewContainer = self.arViewContainer else {
          print("Failed to create ARViewContainer")
          result(FlutterError(code: "AR_VIEW_CREATION_FAILED", message: "Failed to create ARViewContainer", details: nil))
          return
        }
        print("Adding ARViewContainer to RootViewController's view...")
        self.window?.rootViewController?.view.addSubview(arViewContainer)
        print("ARViewContainer added to view hierarchy")
      } else {
        print("ARViewContainer already initialized")
      }
      result(nil)
    }
  }
}
