<template>
    <svg
        class="absolute inset-0 size-full stroke-neutral-900/10 dark:stroke-neutral-100/10"
        fill="none"
    >
        <defs>
            <pattern
                id="pattern-placeholder"
                x="0"
                y="0"
                width="10"
                height="10"
                patternUnits="userSpaceOnUse"
            >
                <path d="M-3 13 15-5M-5 5l18-18M-1 21 17 3" />
            </pattern>
        </defs>
        <rect
            stroke="none"
            fill="url(#pattern-placeholder)"
            width="100%"
            height="100%"
        />
    </svg>
</template>
