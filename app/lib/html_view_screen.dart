// This file conditionally exports the appropriate implementation.
export 'html_view_screen_interface.dart';
export 'html_view_screen_stub.dart'
    if (dart.library.html) 'html_view_screen_web.dart'
    if (dart.library.io) 'html_view_screen_desktop.dart';
