<script setup lang="ts">
import { router } from '@inertiajs/vue3';
import { ref, onMounted } from 'vue';
import { Button } from '@/components/ui/button';
import { useTwoFactorAuth } from '@/composables/useTwoFactorAuth';

const { recoveryCodesList, fetchRecoveryCodes } = useTwoFactorAuth();
const isRecoveryCodesVisible = ref(false);
const regenerating = ref(false);

onMounted(async () => {
    await fetchRecoveryCodes();
});

function toggleRecoveryCodes() {
    isRecoveryCodesVisible.value = !isRecoveryCodesVisible.value;
}

function regenerateRecoveryCodes() {
    regenerating.value = true;
    router.post('/user/two-factor-recovery-codes', {}, {
        preserveScroll: true,
        onSuccess: async () => {
            await fetchRecoveryCodes();
            isRecoveryCodesVisible.value = true;
            regenerating.value = false;
        },
        onError: () => {
            regenerating.value = false;
        },
    });
}
</script>

<template>
    <div class="space-y-4">
        <div class="flex items-center gap-2">
            <Button variant="outline" size="sm" @click="toggleRecoveryCodes">
                {{ isRecoveryCodesVisible ? 'Hide' : 'Show' }} Recovery Codes
            </Button>
            <Button
                variant="outline"
                size="sm"
                :disabled="regenerating"
                @click="regenerateRecoveryCodes"
            >
                Regenerate Recovery Codes
            </Button>
        </div>

        <div
            v-if="isRecoveryCodesVisible && recoveryCodesList.length > 0"
            class="rounded-lg bg-muted p-4"
        >
            <p class="mb-2 text-sm text-muted-foreground">
                Store these recovery codes in a safe place. They can be used to
                recover access to your account if your two-factor authentication
                device is lost.
            </p>
            <div class="grid gap-1 font-mono text-sm">
                <div v-for="code in recoveryCodesList" :key="code">
                    {{ code }}
                </div>
            </div>
        </div>
    </div>
</template>
