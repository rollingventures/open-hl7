<?php

/**
 * Minimal PSR-3 stub for standalone (non-OpenEMR) test runs. In an OpenEMR
 * stack the real psr/log package is autoloaded and this file is never included.
 */

declare(strict_types=1);

namespace Psr\Log {
    interface LoggerInterface
    {
        public function emergency(string|\Stringable $message, array $context = []): void;
        public function alert(string|\Stringable $message, array $context = []): void;
        public function critical(string|\Stringable $message, array $context = []): void;
        public function error(string|\Stringable $message, array $context = []): void;
        public function warning(string|\Stringable $message, array $context = []): void;
        public function notice(string|\Stringable $message, array $context = []): void;
        public function info(string|\Stringable $message, array $context = []): void;
        public function debug(string|\Stringable $message, array $context = []): void;
        public function log($level, string|\Stringable $message, array $context = []): void;
    }

    final class NullLogger implements LoggerInterface
    {
        public function emergency(string|\Stringable $message, array $context = []): void
        {
        }
        public function alert(string|\Stringable $message, array $context = []): void
        {
        }
        public function critical(string|\Stringable $message, array $context = []): void
        {
        }
        public function error(string|\Stringable $message, array $context = []): void
        {
        }
        public function warning(string|\Stringable $message, array $context = []): void
        {
        }
        public function notice(string|\Stringable $message, array $context = []): void
        {
        }
        public function info(string|\Stringable $message, array $context = []): void
        {
        }
        public function debug(string|\Stringable $message, array $context = []): void
        {
        }
        public function log($level, string|\Stringable $message, array $context = []): void
        {
        }
    }
}
