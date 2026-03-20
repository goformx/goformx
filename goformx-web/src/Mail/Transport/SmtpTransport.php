<?php

declare(strict_types=1);

namespace GoFormX\Mail\Transport;

use Waaseyaa\Mail\Envelope;
use Waaseyaa\Mail\Transport\TransportInterface;

/**
 * SMTP transport using PHP socket connection.
 *
 * Sends mail via SMTP (no auth, no TLS) — suitable for local dev
 * with Mailpit or similar relay services.
 */
final class SmtpTransport implements TransportInterface
{
    public function __construct(
        private readonly string $host = 'mailpit',
        private readonly int $port = 1025,
        private readonly int $timeout = 5,
    ) {}

    public function send(Envelope $envelope): void
    {
        $socket = @fsockopen($this->host, $this->port, $errno, $errstr, $this->timeout);
        if ($socket === false) {
            throw new \RuntimeException("SMTP connection failed: {$errstr} ({$errno})");
        }

        try {
            $this->read($socket); // greeting
            $this->command($socket, "EHLO localhost");
            $this->command($socket, "MAIL FROM:<{$envelope->from}>");

            foreach ($envelope->to as $recipient) {
                $this->command($socket, "RCPT TO:<{$recipient}>");
            }

            $this->command($socket, "DATA", 354);

            $headers = $this->buildHeaders($envelope);
            $body = $envelope->htmlBody !== '' ? $envelope->htmlBody : $envelope->textBody;

            fwrite($socket, $headers . "\r\n" . $body . "\r\n.\r\n");
            $this->read($socket); // 250 after DATA

            $this->command($socket, "QUIT", 221);
        } finally {
            fclose($socket);
        }
    }

    private function buildHeaders(Envelope $envelope): string
    {
        $headers = [];
        $headers[] = "From: {$envelope->from}";
        $headers[] = "To: " . implode(', ', $envelope->to);
        $headers[] = "Subject: {$envelope->subject}";
        $headers[] = "Date: " . date('r');
        $headers[] = "MIME-Version: 1.0";

        if ($envelope->htmlBody !== '') {
            $headers[] = "Content-Type: text/html; charset=UTF-8";
        } else {
            $headers[] = "Content-Type: text/plain; charset=UTF-8";
        }

        foreach ($envelope->headers as $name => $value) {
            $headers[] = "{$name}: {$value}";
        }

        return implode("\r\n", $headers);
    }

    /**
     * @param resource $socket
     */
    private function command($socket, string $command, int $expectedCode = 250): string
    {
        fwrite($socket, $command . "\r\n");
        return $this->read($socket, $expectedCode);
    }

    /**
     * @param resource $socket
     */
    private function read($socket, int $expectedCode = 0): string
    {
        $response = '';
        while ($line = fgets($socket, 512)) {
            $response .= $line;
            // Multi-line responses have '-' after code; last line has space
            if (isset($line[3]) && $line[3] === ' ') {
                break;
            }
            // Also break if line is shorter than 4 chars
            if (strlen($line) < 4) {
                break;
            }
        }

        if ($expectedCode > 0) {
            $code = (int) substr($response, 0, 3);
            if ($code !== $expectedCode) {
                throw new \RuntimeException("SMTP error: expected {$expectedCode}, got: {$response}");
            }
        }

        return $response;
    }
}
