// Menu bar icon. AppKit owns the main thread; the agent loop runs in a goroutine
// and pushes state here through menuUpdate. Kept out of main.go's cgo preamble:
// a file with //export may only declare C, not define Objective-C classes.
#import <AppKit/AppKit.h>

extern void goPause(void);
extern void goEdit(void);

static NSStatusItem *item;
static NSMenuItem *inputItem, *pauseItem;

@interface Target : NSObject
@end
@implementation Target
- (void)pause:(id)sender { goPause(); }
- (void)edit:(id)sender { goEdit(); }
@end
static Target *target;

static void setIcon(BOOL paused) {
	NSImage *img = [NSImage imageWithSystemSymbolName:(paused ? @"mic.slash" : @"mic.fill")
	                         accessibilityDescription:(paused ? @"mic-priority, paused" : @"mic-priority")];
	img.template = YES;
	item.button.image = img;
}

// menuRun shows the icon and runs AppKit; it never returns (Quit exits with 0).
void menuRun(void) {
	// Without a window server (e.g. `run` over ssh) AppKit would _Exit(0) on the
	// status item, which launchd takes for Quit: run on without the icon instead.
	CFDictionaryRef gui = CGSessionCopyCurrentDictionary();
	if (!gui) {
		NSLog(@"no GUI session: running without the menu bar icon");
		dispatch_main(); // keeps draining menuUpdate blocks; they no-op on nil items
	}
	CFRelease(gui);

	@autoreleasepool {
		[NSApplication sharedApplication];
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
		target = [Target new];

		NSMenu *menu = [NSMenu new];
		inputItem = [menu addItemWithTitle:@"Input: none" action:nil keyEquivalent:@""]; // no action: shown disabled
		[menu addItem:[NSMenuItem separatorItem]];
		pauseItem = [menu addItemWithTitle:@"Pause Switching" action:@selector(pause:) keyEquivalent:@""];
		pauseItem.target = target;
		[menu addItemWithTitle:@"Edit Priority List…" action:@selector(edit:) keyEquivalent:@""].target = target;
		[menu addItem:[NSMenuItem separatorItem]];
		[menu addItemWithTitle:@"Quit mic-priority" action:@selector(terminate:) keyEquivalent:@""];

		item = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
		item.menu = menu;
		setIcon(NO);
	}
	[NSApp run];
}

// menuUpdate may be called from any thread, also before menuRun: the main queue
// runs the block once [NSApp run] starts.
void menuUpdate(const char *name, int paused) {
	NSString *s = [NSString stringWithUTF8String:name] ?: @"";
	dispatch_async(dispatch_get_main_queue(), ^{
		inputItem.title = [@"Input: " stringByAppendingString:(s.length ? s : @"none")];
		pauseItem.state = paused ? NSControlStateValueOn : NSControlStateValueOff;
		setIcon(paused);
	});
}
