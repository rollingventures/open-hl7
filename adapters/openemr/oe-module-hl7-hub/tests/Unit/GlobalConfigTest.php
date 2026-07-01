<?php

declare(strict_types=1);

namespace OpenEMR\Modules\Hl7Hub\Tests\Unit;

use OpenEMR\Modules\Hl7Hub\GlobalConfig;
use PHPUnit\Framework\TestCase;

final class GlobalConfigTest extends TestCase
{
    protected function tearDown(): void
    {
        putenv('HL7HUB_URL');
        putenv('HL7HUB_SECRET');
    }

    public function testDisabledWhenNoUrl(): void
    {
        putenv('HL7HUB_URL');
        $c = GlobalConfig::fromEnvironment();
        self::assertFalse($c->isEnabled());
        self::assertSame('', $c->hubUrl);
    }

    public function testEnabledAndTrimsUrl(): void
    {
        putenv('HL7HUB_URL=  http://hub:8088  ');
        putenv('HL7HUB_SECRET=s3cr3t');
        $c = GlobalConfig::fromEnvironment();

        self::assertTrue($c->isEnabled());
        self::assertSame('http://hub:8088', $c->hubUrl);
        self::assertSame('s3cr3t', $c->secret);
    }

    public function testEventsEndpointStripsTrailingSlash(): void
    {
        putenv('HL7HUB_URL=http://hub:8088/');
        self::assertSame('http://hub:8088/events', GlobalConfig::fromEnvironment()->eventsEndpoint());
    }

    public function testSecretDefaultsToEmpty(): void
    {
        putenv('HL7HUB_URL=http://hub:8088');
        putenv('HL7HUB_SECRET');
        self::assertSame('', GlobalConfig::fromEnvironment()->secret);
    }
}
