<script setup lang="ts">
import { router } from '@inertiajs/vue3';
import { ref, watch, onMounted } from 'vue';
import InputError from '@/components/InputError.vue';
import { Button } from '@/components/ui/button';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { InputOTP, InputOTPGroup, InputOTPSlot } from '@/components/ui/input-otp';
import { Label } from '@/components/ui/label';
import { useTwoFactorAuth } from '@/composables/useTwoFactorAuth';

const props = defineProps<{
    isOpen: boolean;
    requiresConfirmation: boolean;
    twoFactorEnabled: boolean;
}>();

const emit = defineEmits<{
    'update:isOpen': [value: boolean];
}>();

const {
    qrCodeSvg,
    manualSetupKey,
    recoveryCodesList,
    errors: setupErrors,
    fetchSetupData,
    fetchRecoveryCodes,
    clearErrors,
} = useTwoFactorAuth();

const step = ref<'qr' | 'verify' | 'recovery'>('qr');
const otpCode = ref('');
const verifying = ref(false);
const verifyError = ref('');

watch(
    () => props.isOpen,
    async (open) => {
        if (open) {
            step.value = 'qr';
            otpCode.value = '';
            verifyError.value = '';
            clearErrors();
            await fetchSetupData();
        }
    },
);

function handleNextStep() {
    if (step.value === 'qr') {
        if (props.requiresConfirmation && !props.twoFactorEnabled) {
            step.value = 'verify';
        } else {
            step.value = 'recovery';
            void fetchRecoveryCodes();
        }
    } else if (step.value === 'verify') {
        confirmTwoFactor();
    } else {
        emit('update:isOpen', false);
    }
}

function confirmTwoFactor() {
    verifying.value = true;
    verifyError.value = '';
    router.post(
        '/user/confirmed-two-factor-authentication',
        { code: otpCode.value },
        {
            preserveScroll: true,
            onSuccess: () => {
                verifying.value = false;
                step.value = 'recovery';
                void fetchRecoveryCodes();
            },
            onError: (errs) => {
                verifying.value = false;
                verifyError.value =
                    errs.code ?? 'Invalid verification code. Please try again.';
            },
        },
    );
}
</script>

<template>
    <Dialog :open="isOpen" @update:open="emit('update:isOpen', $event)">
        <DialogContent class="sm:max-w-md">
            <DialogHeader>
                <DialogTitle>
                    <template v-if="step === 'qr'"
                        >Scan QR Code</template
                    >
                    <template v-else-if="step === 'verify'"
                        >Verify Setup</template
                    >
                    <template v-else>Recovery Codes</template>
                </DialogTitle>
                <DialogDescription>
                    <template v-if="step === 'qr'">
                        Scan this QR code with your authenticator app, or enter
                        the setup key manually.
                    </template>
                    <template v-else-if="step === 'verify'">
                        Enter the 6-digit code from your authenticator app to
                        verify setup.
                    </template>
                    <template v-else>
                        Store these recovery codes in a secure location. They
                        can be used to access your account if you lose your
                        authenticator device.
                    </template>
                </DialogDescription>
            </DialogHeader>

            <!-- QR Code Step -->
            <div v-if="step === 'qr'" class="space-y-4">
                <div
                    v-if="qrCodeSvg"
                    class="flex justify-center"
                    v-html="qrCodeSvg"
                />
                <div
                    v-if="manualSetupKey"
                    class="rounded-lg bg-muted p-3 text-center"
                >
                    <Label class="text-xs text-muted-foreground"
                        >Setup Key</Label
                    >
                    <p class="mt-1 font-mono text-sm">{{ manualSetupKey }}</p>
                </div>
                <div
                    v-if="!qrCodeSvg && !manualSetupKey"
                    class="flex h-48 items-center justify-center"
                >
                    <div class="h-8 w-8 animate-pulse rounded-full bg-muted" />
                </div>
            </div>

            <!-- Verify Step -->
            <div v-else-if="step === 'verify'" class="space-y-4">
                <div class="flex justify-center">
                    <InputOTP v-model="otpCode" :num-inputs="6">
                        <InputOTPGroup>
                            <InputOTPSlot
                                v-for="i in 6"
                                :key="i"
                                :index="i - 1"
                            />
                        </InputOTPGroup>
                    </InputOTP>
                </div>
                <InputError :message="verifyError" />
            </div>

            <!-- Recovery Codes Step -->
            <div v-else class="space-y-4">
                <div
                    v-if="recoveryCodesList.length > 0"
                    class="rounded-lg bg-muted p-4"
                >
                    <div class="grid gap-1 font-mono text-sm">
                        <div v-for="code in recoveryCodesList" :key="code">
                            {{ code }}
                        </div>
                    </div>
                </div>
                <div v-else class="flex h-24 items-center justify-center">
                    <div class="h-8 w-8 animate-pulse rounded-full bg-muted" />
                </div>
            </div>

            <DialogFooter>
                <Button @click="handleNextStep" :disabled="verifying">
                    <template v-if="step === 'qr'">Next</template>
                    <template v-else-if="step === 'verify'">
                        {{ verifying ? 'Verifying...' : 'Verify' }}
                    </template>
                    <template v-else>Done</template>
                </Button>
            </DialogFooter>
        </DialogContent>
    </Dialog>
</template>
