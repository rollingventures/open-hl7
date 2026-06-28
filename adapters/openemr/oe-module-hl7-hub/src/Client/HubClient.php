<?php

/**
 * @package   OpenEMR
 * @link      https://www.open-emr.org
 * @author    Chris Dickman
 * @copyright Copyright (c) 2026 Chris Dickman
 * @license   https://github.com/openemr/openemr/blob/master/LICENSE GNU General Public License 3
 */

declare(strict_types=1);

namespace OpenEMR\Modules\Hl7Hub\Client;

use OpenEMR\Modules\Hl7Hub\GlobalConfig;
use Psr\Log\LoggerInterface;
use RuntimeException;

/**
 * Thin HTTP client that POSTs canonical events to the hub control-plane,
 * authenticated with the shared secret in the X-Hub-Secret header.
 */
final class HubClient
{
    private const TIMEOUT_SECONDS = 5;

    public function __construct(
        private readonly GlobalConfig $config,
        private readonly LoggerInterface $logger,
    ) {
    }

    /**
     * @param array<string, mixed> $canonicalPatient
     *
     * @throws RuntimeException on transport failure or non-2xx response
     */
    public function sendPatient(array $canonicalPatient): void
    {
        $body = json_encode($canonicalPatient, JSON_THROW_ON_ERROR);

        $ch = curl_init($this->config->eventsEndpoint());
        if ($ch === false) {
            throw new RuntimeException('Unable to initialize HTTP client');
        }

        curl_setopt_array($ch, [
            CURLOPT_POST => true,
            CURLOPT_POSTFIELDS => $body,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_CONNECTTIMEOUT => self::TIMEOUT_SECONDS,
            CURLOPT_TIMEOUT => self::TIMEOUT_SECONDS,
            CURLOPT_HTTPHEADER => [
                'Content-Type: application/json',
                'X-Hub-Secret: ' . $this->config->secret,
            ],
        ]);

        $response = curl_exec($ch);
        $status = (int) curl_getinfo($ch, CURLINFO_RESPONSE_CODE);
        $error = curl_error($ch);
        curl_close($ch);

        if ($response === false) {
            // Generic message to caller; details only to the log.
            $this->logger->warning('HL7 hub unreachable', ['error' => $error]);
            throw new RuntimeException('HL7 hub request failed');
        }

        if ($status < 200 || $status >= 300) {
            $this->logger->warning('HL7 hub rejected event', ['status' => $status]);
            throw new RuntimeException('HL7 hub returned status ' . $status);
        }

        $this->logger->debug('HL7 hub accepted event', ['status' => $status]);
    }
}
