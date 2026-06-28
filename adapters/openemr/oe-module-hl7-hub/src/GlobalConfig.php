<?php

/**
 * @package   OpenEMR
 * @link      https://www.open-emr.org
 * @author    Chris Dickman
 * @copyright Copyright (c) 2026 Chris Dickman
 * @license   https://github.com/openemr/openemr/blob/master/LICENSE GNU General Public License 3
 */

declare(strict_types=1);

namespace OpenEMR\Modules\Hl7Hub;

/**
 * Module configuration: the hub base URL and the shared secret used to sign the
 * adapter <-> hub HTTP traffic.
 *
 * M1 reads from environment variables (HL7HUB_URL, HL7HUB_SECRET) so the module
 * is usable before the admin settings UI lands. A later increment registers
 * these as OpenEMR globals via GlobalsInitializedEvent.
 */
final readonly class GlobalConfig
{
    public function __construct(
        public string $hubUrl,
        public string $secret,
    ) {
    }

    public static function fromEnvironment(): self
    {
        $url = getenv('HL7HUB_URL');
        $secret = getenv('HL7HUB_SECRET');

        return new self(
            is_string($url) ? trim($url) : '',
            is_string($secret) ? $secret : '',
        );
    }

    public function isEnabled(): bool
    {
        return $this->hubUrl !== '';
    }

    public function eventsEndpoint(): string
    {
        return rtrim($this->hubUrl, '/') . '/events';
    }
}
