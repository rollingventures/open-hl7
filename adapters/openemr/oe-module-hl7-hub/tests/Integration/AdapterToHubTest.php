<?php

declare(strict_types=1);

namespace OpenEMR\Modules\Hl7Hub\Tests\Integration;

use OpenEMR\Modules\Hl7Hub\Canonical\PatientMapper;
use OpenEMR\Modules\Hl7Hub\Client\HubClient;
use OpenEMR\Modules\Hl7Hub\GlobalConfig;
use PHPUnit\Framework\TestCase;
use Psr\Log\NullLogger;

/**
 * End-to-end: map an OpenEMR patient row -> canonical -> real HubClient POST to
 * a running open-hl7 hub, then assert the hub encoded + archived the ADT.
 *
 * Skipped unless HUB_BASE points at a running hub (e.g. http://127.0.0.1:8088).
 * The runner script (tests/run-integration.sh) starts the hub for this.
 */
final class AdapterToHubTest extends TestCase
{
    private string $base;

    protected function setUp(): void
    {
        $base = getenv('HUB_BASE');
        if ($base === false || $base === '') {
            self::markTestSkipped('set HUB_BASE (e.g. http://127.0.0.1:8088) to run the adapter->hub integration test');
        }
        $this->base = rtrim($base, '/');
    }

    public function testMappedPatientIsEncodedAndArchivedByHub(): void
    {
        putenv('HL7HUB_URL=' . $this->base);
        putenv('HL7HUB_SECRET=' . (getenv('HUB_SECRET') ?: ''));

        $client = new HubClient(GlobalConfig::fromEnvironment(), new NullLogger());

        $mrn = 'IT' . random_int(10000, 99999);
        $row = [
            'pubpid' => $mrn,
            'lname' => 'Tester',
            'fname' => 'Ada',
            'DOB' => '1975-12-10',
            'sex' => 'Female',
            'street' => '9 Elm',
            'city' => 'Austin',
            'state' => 'TX',
            'postal_code' => '78702',
            'country_code' => 'US',
            'phone_home' => '555-9000',
        ];

        // Real adapter code path: map, then POST via the real HubClient.
        $client->sendPatient(PatientMapper::toCanonical($row, 'create'));

        $messages = $this->fetchMessages();
        $match = null;
        foreach ($messages as $m) {
            if (($m['Direction'] ?? '') === 'outbound' && str_contains((string) ($m['Raw'] ?? ''), $mrn)) {
                $match = $m;
                break;
            }
        }

        self::assertNotNull($match, 'hub did not archive a message containing MRN ' . $mrn);
        self::assertSame('ADT^A04', $match['Type'], 'create event should encode as ADT^A04');
        self::assertStringContainsString($mrn . '^^^OPENEMR^MR', $match['Raw'], 'PID-3 should carry the MRN');
        self::assertStringContainsString('Tester^Ada', $match['Raw'], 'PID-5 should carry the name');
        self::assertStringContainsString('|19751210|F|', $match['Raw'], 'DOB + sex should be HL7-encoded');
    }

    /** @return array<int, array<string, mixed>> */
    private function fetchMessages(): array
    {
        $json = file_get_contents($this->base . '/messages');
        self::assertNotFalse($json, 'could not read /messages from hub');
        $decoded = json_decode($json, true);
        self::assertIsArray($decoded);

        return $decoded;
    }
}
