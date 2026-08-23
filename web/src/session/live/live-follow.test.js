import { describe, expect, it, vi } from 'vitest';
import { JSDOM } from 'jsdom';
import { createFollowScrollController } from './live-follow.js';

function setup({ scrollHeight = 2000, innerHeight = 1000 } = {}) {
  const dom = new JSDOM('<body><main id="content"></main></body>');
  const documentImpl = dom.window.document;
  Object.defineProperty(documentImpl.documentElement, 'scrollHeight', {
    value: scrollHeight,
    configurable: true,
  });
  Object.defineProperty(documentImpl.body, 'scrollHeight', {
    value: scrollHeight,
    configurable: true,
  });

  const handlers = {};
  const windowImpl = {
    scrollY: 0,
    pageYOffset: 0,
    innerHeight,
    scrollTo: vi.fn(),
    setTimeout: (cb) => {
      cb();
      return 0;
    },
    requestAnimationFrame: (cb) => {
      cb();
      return 0;
    },
    addEventListener: (type, handler) => {
      (handlers[type] ||= []).push(handler);
    },
    removeEventListener: (type, handler) => {
      handlers[type] = (handlers[type] || []).filter((h) => h !== handler);
    },
  };
  const fire = (type, extra = {}) => (handlers[type] || []).forEach((h) => h({ type, ...extra }));

  const controller = createFollowScrollController({
    documentImpl,
    windowImpl,
    requestAnimationFrameImpl: (cb) => {
      cb();
      return 0;
    },
    setTimeoutImpl: (cb) => {
      cb();
      return 0;
    },
  });
  return { dom, documentImpl, windowImpl, handlers, fire, controller };
}

describe('createFollowScrollController', () => {
  it('starts following and scrolls to bottom on init', () => {
    const { windowImpl, controller } = setup();
    expect(controller.isFollowing()).toBe(true);
    expect(controller.shouldFollow()).toBe(true);
    expect(windowImpl.scrollTo).toHaveBeenCalledTimes(1);
  });

  it('stops following and shows the follow button when scrolled away from bottom', () => {
    const { documentImpl, windowImpl, fire, controller } = setup();
    windowImpl.scrollY = 0; // remaining = 2000 - 0 - 1000 = 1000 (> threshold)
    fire('scroll');
    expect(controller.isFollowing()).toBe(false);
    expect(documentImpl.querySelector('.follow-button')).not.toBeNull();
  });

  it('keeps following on a downward scrollTop clamp that stays at the bottom', () => {
    const { documentImpl, windowImpl, fire, controller } = setup();
    windowImpl.scrollY = 1000; // remaining = 2000 - 1000 - 1000 = 0 (at bottom)
    fire('scroll');
    expect(controller.isFollowing()).toBe(true);
    // The document shrinks when the streaming preview is finalized/removed, so
    // the browser clamps scrollTop downward — but we are still at the bottom.
    windowImpl.scrollY = 960; // remaining = 40 (< 80 threshold, still at bottom)
    fire('scroll');
    expect(controller.isFollowing()).toBe(true);
    expect(documentImpl.querySelector('.follow-button')).toBeNull();
  });

  it('releases forced follow when the user scrolls up away from the bottom', () => {
    const { windowImpl, fire, controller } = setup();
    windowImpl.scrollY = 1000;
    fire('scroll');
    controller.extendPreviewFollow(30000);
    expect(controller.shouldFollow()).toBe(true);
    windowImpl.scrollY = 0; // remaining = 1000 (away from bottom) and scrolled up
    fire('scroll');
    expect(controller.isFollowing()).toBe(false);
    expect(controller.shouldFollow()).toBe(false);
  });

  it('clicking the follow button re-follows and removes the button', () => {
    const { documentImpl, windowImpl, fire, controller } = setup();
    fire('scroll');
    const btn = documentImpl.querySelector('.follow-button');
    expect(btn).not.toBeNull();
    windowImpl.scrollTo.mockClear();
    btn.click();
    expect(windowImpl.scrollTo).toHaveBeenCalledWith({ top: 2000, behavior: 'smooth' });
    expect(documentImpl.querySelector('.follow-button')).toBeNull();
    expect(controller.isFollowing()).toBe(true);
  });

  it('extendPreviewFollow keeps shouldFollow true while not following', () => {
    const { fire, controller } = setup();
    fire('scroll'); // following becomes false
    expect(controller.shouldFollow()).toBe(false);
    controller.extendPreviewFollow(30000);
    expect(controller.shouldFollow()).toBe(true);
  });

  it('forceFollowToBottom re-follows and scrolls', () => {
    const { windowImpl, fire, controller } = setup();
    fire('scroll');
    expect(controller.isFollowing()).toBe(false);
    windowImpl.scrollTo.mockClear();
    controller.forceFollowToBottom(true);
    expect(controller.isFollowing()).toBe(true);
    expect(windowImpl.scrollTo).toHaveBeenCalled();
  });

  it('ignores non-scrolling keys for follow decisions', () => {
    const { fire, controller } = setup();
    fire('keydown', { key: 'a' });
    expect(controller.isFollowing()).toBe(true);
  });

  it('dispose removes listeners so later scrolls no longer change state', () => {
    const { fire, controller } = setup();
    controller.dispose();
    fire('scroll');
    expect(controller.isFollowing()).toBe(true);
  });

  it('keeps following after a reflow-induced scroll-down on #content', () => {
    // Regression: when streaming preview clears and #content re-renders, the
    // scroll container shrinks and the browser clamps scrollTop downward.
    // That fires a scroll event with currentScroll < lastScrollTop even though
    // the user is still parked at the bottom — follow mode must NOT drop.
    const dom = new JSDOM('<body><main id="content"></main></body>');
    const documentImpl = dom.window.document;
    const content = documentImpl.getElementById('content');

    // Window is NOT scrollable so isAtBottom falls through to #content path.
    Object.defineProperty(documentImpl.documentElement, 'scrollHeight', {
      value: 100, // <= innerHeight => not window-scrollable
      configurable: true,
    });
    Object.defineProperty(documentImpl.body, 'scrollHeight', {
      value: 100,
      configurable: true,
    });

    // #content IS scrollable: scrollHeight 1000, clientHeight 100, scrollTop 900
    // => remaining = 1000 - 900 - 100 = 0 < 100 => at bottom
    Object.defineProperty(content, 'scrollHeight', { value: 1000, configurable: true });
    Object.defineProperty(content, 'clientHeight', { value: 100, configurable: true });
    Object.defineProperty(content, 'scrollTop', { value: 900, configurable: true });
    content.scrollTo = vi.fn();

    const handlers = {};
    const contentHandlers = {};
    const windowImpl = {
      scrollY: 0,
      pageYOffset: 0,
      innerHeight: 600,
      scrollTo: vi.fn(),
      setTimeout: (cb) => {
        cb();
        return 0;
      },
      requestAnimationFrame: (cb) => {
        cb();
        return 0;
      },
      addEventListener: (type, handler) => {
        (handlers[type] ||= []).push(handler);
      },
      removeEventListener: (type, handler) => {
        handlers[type] = (handlers[type] || []).filter((h) => h !== handler);
      },
    };

    // Monkey-patch content addEventListener so we can fire scroll on it.
    const origAdd = content.addEventListener.bind(content);
    content.addEventListener = (type, handler) => {
      origAdd(type, handler);
      (contentHandlers[type] ||= []).push(handler);
    };

    const controller = createFollowScrollController({
      documentImpl,
      windowImpl,
      requestAnimationFrameImpl: (cb) => {
        cb();
        return 0;
      },
      setTimeoutImpl: (cb) => {
        cb();
        return 0;
      },
    });

    // Initial state — user is at bottom, following.
    expect(controller.isFollowing()).toBe(true);

    // Simulate reflow: content shrinks from 1000 -> 950, scrollTop clamped 900 -> 850.
    // User is STILL at bottom: 950 - 850 - 100 = 0 < 100.
    // But currentScroll (850) < lastScrollTop (900) => scrolledUp = true.
    Object.defineProperty(content, 'scrollHeight', { value: 950, configurable: true });
    Object.defineProperty(content, 'scrollTop', { value: 850, configurable: true });

    // Fire scroll on #content (the actual scroll container in this path).
    (contentHandlers['scroll'] || []).forEach((h) => h({ type: 'scroll' }));

    // Must still be following — this is the assertion that fails before the fix.
    expect(controller.isFollowing()).toBe(true);
  });

  it('re-pins to the bottom when observed content grows while following', () => {
    // The ResizeObserver fires AFTER the DOM grows, which is what lets us follow
    // streaming output regardless of when the async render lands.
    let roCb = null;
    const observed = [];
    class FakeResizeObserver {
      constructor(cb) {
        roCb = cb;
      }
      observe(el) {
        observed.push(el);
      }
      disconnect() {}
    }

    const dom = new JSDOM(
      '<body><main id="content"><div id="messages"></div><div id="chat-preview-host"></div></main></body>',
      { url: 'http://localhost/' },
    );
    const documentImpl = dom.window.document;
    Object.defineProperty(documentImpl.documentElement, 'scrollHeight', {
      value: 2000,
      configurable: true,
    });
    Object.defineProperty(documentImpl.body, 'scrollHeight', { value: 2000, configurable: true });

    const windowImpl = {
      scrollY: 1000, // at bottom: 2000 - 1000 - 1000 = 0
      pageYOffset: 1000,
      innerHeight: 1000,
      scrollTo: vi.fn(),
      setTimeout: (cb) => {
        cb();
        return 0;
      },
      requestAnimationFrame: (cb) => {
        cb();
        return 0;
      },
      addEventListener: () => {},
      removeEventListener: () => {},
      ResizeObserver: FakeResizeObserver,
    };

    createFollowScrollController({
      documentImpl,
      windowImpl,
      requestAnimationFrameImpl: (cb) => {
        cb();
        return 0;
      },
      setTimeoutImpl: (cb) => {
        cb();
        return 0;
      },
    });

    expect(observed).toContain(documentImpl.getElementById('messages'));
    expect(observed).toContain(documentImpl.getElementById('chat-preview-host'));

    windowImpl.scrollTo.mockClear();
    roCb(); // content grew below the fold
    expect(windowImpl.scrollTo).toHaveBeenCalled();
  });

  it('does not re-pin on content growth after the user scrolls up', () => {
    let roCb = null;
    class FakeResizeObserver {
      constructor(cb) {
        roCb = cb;
      }
      observe() {}
      disconnect() {}
    }

    const dom = new JSDOM('<body><main id="content"></main></body>');
    const documentImpl = dom.window.document;
    Object.defineProperty(documentImpl.documentElement, 'scrollHeight', {
      value: 2000,
      configurable: true,
    });
    Object.defineProperty(documentImpl.body, 'scrollHeight', { value: 2000, configurable: true });

    const handlers = {};
    const windowImpl = {
      scrollY: 0, // NOT at bottom: 2000 - 0 - 1000 = 1000
      pageYOffset: 0,
      innerHeight: 1000,
      scrollTo: vi.fn(),
      setTimeout: (cb) => {
        cb();
        return 0;
      },
      requestAnimationFrame: (cb) => {
        cb();
        return 0;
      },
      addEventListener: (type, handler) => {
        (handlers[type] ||= []).push(handler);
      },
      removeEventListener: () => {},
      ResizeObserver: FakeResizeObserver,
    };

    const controller = createFollowScrollController({
      documentImpl,
      windowImpl,
      requestAnimationFrameImpl: (cb) => {
        cb();
        return 0;
      },
      setTimeoutImpl: (cb) => {
        cb();
        return 0;
      },
    });

    (handlers['scroll'] || []).forEach((h) => h({ type: 'scroll' }));
    expect(controller.isFollowing()).toBe(false);

    windowImpl.scrollTo.mockClear();
    roCb(); // content grows, but the user has scrolled up
    expect(windowImpl.scrollTo).not.toHaveBeenCalled();
  });

  it('a wheel-up stops following and is not yanked back as content streams in', () => {
    let roCb = null;
    class FakeResizeObserver {
      constructor(cb) {
        roCb = cb;
      }
      observe() {}
      disconnect() {}
    }

    const dom = new JSDOM('<body><main id="content"></main></body>', { url: 'http://localhost/' });
    const documentImpl = dom.window.document;
    Object.defineProperty(documentImpl.documentElement, 'scrollHeight', {
      value: 2000,
      configurable: true,
    });
    Object.defineProperty(documentImpl.body, 'scrollHeight', { value: 2000, configurable: true });

    const handlers = {};
    const windowImpl = {
      scrollY: 1000, // start AT the bottom (2000 - 1000 - 1000 = 0)
      pageYOffset: 1000,
      innerHeight: 1000,
      scrollTo: vi.fn(),
      setTimeout: (cb) => {
        cb();
        return 0;
      },
      requestAnimationFrame: (cb) => {
        cb();
        return 0;
      },
      addEventListener: (type, handler) => {
        (handlers[type] ||= []).push(handler);
      },
      removeEventListener: () => {},
      ResizeObserver: FakeResizeObserver,
    };

    const controller = createFollowScrollController({
      documentImpl,
      windowImpl,
      requestAnimationFrameImpl: (cb) => {
        cb();
        return 0;
      },
    });
    expect(controller.isFollowing()).toBe(true);

    // User wheels up — follow stops immediately, regardless of position timing.
    (handlers['wheel'] || []).forEach((h) => h({ type: 'wheel', deltaY: -200 }));
    expect(controller.isFollowing()).toBe(false);
    expect(documentImpl.querySelector('.follow-button')).not.toBeNull();

    // Streaming content keeps growing; the re-pin must be suppressed.
    windowImpl.scrollTo.mockClear();
    roCb();
    expect(windowImpl.scrollTo).not.toHaveBeenCalled();
  });
});
