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
        if (this.isDevelopment) {
            console.error(...args);
        }
    }

    static warn(...args: unknown[]): void {
        if (this.isDevelopment) {
            console.warn(...args);
        }
    }

    static debug(...args: unknown[]): void {
        if (this.isDevelopment) {
            console.log(...args);
        }
    }
}
