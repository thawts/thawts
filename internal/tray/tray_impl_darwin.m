#include "_cgo_export.h"
#import <Cocoa/Cocoa.h>

@interface TrayTarget : NSObject
- (void)onShow:(id)sender;
- (void)onQuit:(id)sender;
- (void)onExport:(id)sender;
- (void)onImport:(id)sender;
@end

@implementation TrayTarget
- (void)onShow:(id)sender {
  handleTrayClick(1);
}
- (void)onQuit:(id)sender {
  handleTrayClick(2);
}
- (void)onExport:(id)sender {
  handleTrayClick(3);
}
- (void)onImport:(id)sender {
  handleTrayClick(4);
}
@end

static TrayTarget *target;
static NSStatusItem *statusItem;

void setupTray(void) {
  // Ensure we are on the main thread for UI operations
  dispatch_async(dispatch_get_main_queue(), ^{
    target = [[TrayTarget alloc] init];

    statusItem = [[NSStatusBar systemStatusBar]
        statusItemWithLength:NSVariableStatusItemLength];
    [statusItem retain]; // Prevent deallocation

    NSImage *icon = [NSApplication sharedApplication].applicationIconImage;
    if (icon) {
      // Resize icon to fit menu bar (usually 18x18 or 22x22)
      NSSize newSize = NSMakeSize(18, 18);
      NSImage *smallIcon = [[NSImage alloc] initWithSize:newSize];
      [smallIcon lockFocus];
      [icon drawInRect:NSMakeRect(0, 0, newSize.width, newSize.height)
              fromRect:NSZeroRect
             operation:NSCompositingOperationCopy
              fraction:1.0];
      [smallIcon unlockFocus];

      statusItem.button.image = smallIcon;
    } else {
      statusItem.button.title = @"T";
    }

    NSMenu *menu = [[NSMenu alloc] init];

    NSMenuItem *showItem = [[NSMenuItem alloc] initWithTitle:@"Show Thawts"
                                                      action:@selector(onShow:)
                                               keyEquivalent:@""];
    [showItem setTarget:target];
    [menu addItem:showItem];

    [menu addItem:[NSMenuItem separatorItem]];

    NSMenuItem *exportItem =
        [[NSMenuItem alloc] initWithTitle:@"Export Data..."
                                   action:@selector(onExport:)
                            keyEquivalent:@""];
    [exportItem setTarget:target];
    [menu addItem:exportItem];

    NSMenuItem *importItem =
        [[NSMenuItem alloc] initWithTitle:@"Import Data..."
                                   action:@selector(onImport:)
                            keyEquivalent:@""];
    [importItem setTarget:target];
    [menu addItem:importItem];

    [menu addItem:[NSMenuItem separatorItem]];

    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Quit"
                                                      action:@selector(onQuit:)
                                               keyEquivalent:@"q"];
    [quitItem setTarget:target];
    [menu addItem:quitItem];

    statusItem.menu = menu;
  });
}
