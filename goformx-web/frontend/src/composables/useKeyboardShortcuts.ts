import { onMounted, onUnmounted, type Ref, ref } from 'vue';

export interface ShortcutConfig {
    key: string;
    ctrl?: boolean;
    meta?: boolean;
    shift?: boolean;
    alt?: boolean;
    handler: () => void;
    description: string;
    preventDefault?: boolean;
}

export interface UseKeyboardShortcutsOptions {
    enabled?: Ref<boolean>;
}

export interface UseKeyboardShortcutsReturn {
    shortcuts: Ref<ShortcutConfig[]>;
    isEnabled: Ref<boolean>;
    enable: () => void;
    disable: () => void;
    toggle: () => void;
}

export function useKeyboardShortcuts(
    shortcuts: ShortcutConfig[],
    options?: UseKeyboardShortcutsOptions,
): UseKeyboardShortcutsReturn {
    const isEnabled = options?.enabled ?? ref(true);
    const isMac = ref(false);

    onMounted(() => {
        isMac.value = navigator.platform.toLowerCase().includes('mac');
    });

    const matchesShortcut = (
        event: KeyboardEvent,
        shortcut: ShortcutConfig,
    ): boolean => {
        const keyMatches =
            event.key.toLowerCase() === shortcut.key.toLowerCase();
        const ctrlMatches = shortcut.ctrl ? event.ctrlKey : !event.ctrlKey;
        const metaMatches = shortcut.meta ? event.metaKey : !event.metaKey;
        const shiftMatches = shortcut.shift ? event.shiftKey : !event.shiftKey;
        const altMatches = shortcut.alt ? event.altKey : !event.altKey;
        return (
            keyMatches &&
            ctrlMatches &&
            metaMatches &&
            shiftMatches &&
            altMatches
        );
    };

    const handleKeyDown = (event: KeyboardEvent): void => {
        if (!isEnabled.value) return;
        const matchedShortcut = shortcuts.find((shortcut) =>
            matchesShortcut(event, shortcut),
        );
        if (matchedShortcut) {
            if (matchedShortcut.preventDefault !== false) {
                event.preventDefault();
            }
            matchedShortcut.handler();
        }
    };

    onMounted(() => {
        window.addEventListener('keydown', handleKeyDown);
    });

    onUnmounted(() => {
        window.removeEventListener('keydown', handleKeyDown);
    });

    const enable = (): void => {
        if (typeof isEnabled.value === 'boolean') {
            (isEnabled as Ref<boolean>).value = true;
        }
    };

    const disable = (): void => {
        if (typeof isEnabled.value === 'boolean') {
            (isEnabled as Ref<boolean>).value = false;
        }
    };

    const toggle = (): void => {
        if (typeof isEnabled.value === 'boolean') {
            (isEnabled as Ref<boolean>).value = !(isEnabled as Ref<boolean>)
                .value;
        }
    };

    return {
        shortcuts: ref(shortcuts),
        isEnabled,
        enable,
        disable,
        toggle,
    };
}

export function formatShortcut(shortcut: ShortcutConfig): string {
    const isMac = navigator.platform.toLowerCase().includes('mac');
    const parts: string[] = [];
    if (shortcut.ctrl) parts.push('Ctrl');
    if (shortcut.alt) parts.push(isMac ? '⌥' : 'Alt');
    if (shortcut.shift) parts.push(isMac ? '⇧' : 'Shift');
    if (shortcut.meta) parts.push(isMac ? '⌘' : 'Ctrl');
    parts.push(shortcut.key.toUpperCase());
    return parts.join(isMac ? '' : '+');
}
