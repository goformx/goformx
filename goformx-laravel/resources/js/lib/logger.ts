/**
 * Logger utility for conditional logging based on environment
 */
export class Logger {
    private static readonly isDevelopment = import.meta.env.DEV;

    static log(...args: unknown[]): void {
        if (this.isDevelopment) {
            console.log(...args);
        }
    }

    static error(...args: unknown[]): void {
        // Errors always log regardless of environment
        console.error(...args);
    }

    static warn(...args: unknown[]): void {
        // Warnings always log regardless of environment
        console.warn(...args);
    }

    static debug(...args: unknown[]): void {
        if (this.isDevelopment) {
            console.log(...args);
        }
    }
}
