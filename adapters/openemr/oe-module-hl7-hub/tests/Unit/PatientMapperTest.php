<?php

declare(strict_types=1);

namespace OpenEMR\Modules\Hl7Hub\Tests\Unit;

use OpenEMR\Modules\Hl7Hub\Canonical\PatientMapper;
use PHPUnit\Framework\Attributes\DataProvider;
use PHPUnit\Framework\TestCase;

final class PatientMapperTest extends TestCase
{
    /** @return array<string, mixed> a representative OpenEMR patient_data row */
    private static function openEmrRow(): array
    {
        return [
            'pubpid' => '12345',
            'pid' => '7',
            'lname' => 'Doe',
            'fname' => 'Jane',
            'mname' => 'Q',
            'DOB' => '1980-01-15',
            'sex' => 'Female',
            'street' => '1 Main St',
            'city' => 'Austin',
            'state' => 'TX',
            'postal_code' => '78701',
            'country_code' => 'US',
            'phone_home' => '555-1234',
        ];
    }

    public function testToCanonicalMapsCoreFields(): void
    {
        $c = PatientMapper::toCanonical(self::openEmrRow(), 'create');

        self::assertSame('12345', $c['mrn']);
        self::assertSame('Doe', $c['familyName']);
        self::assertSame('Jane', $c['givenName']);
        self::assertSame('Q', $c['middleName']);
        self::assertSame('19800115', $c['birthDate'], 'DOB should become HL7 YYYYMMDD');
        self::assertSame('F', $c['sex'], 'Female should map to HL7 F');
        self::assertSame('create', $c['event']);
        self::assertSame('openemr', $c['source']['emr']);
        self::assertSame('1 Main St', $c['address']['line1']);
        self::assertSame('Austin', $c['address']['city']);
        self::assertSame('78701', $c['address']['zip']);
        self::assertSame('555-1234', $c['phone']);
    }

    public function testMrnFallsBackToPidWhenNoPubpid(): void
    {
        $row = self::openEmrRow();
        unset($row['pubpid']);
        $c = PatientMapper::toCanonical($row, 'update');
        self::assertSame('7', $c['mrn']);
        self::assertSame('update', $c['event']);
    }

    #[DataProvider('sexProvider')]
    public function testSexNormalization(string $openemr, string $hl7): void
    {
        $row = self::openEmrRow();
        $row['sex'] = $openemr;
        self::assertSame($hl7, PatientMapper::toCanonical($row, 'create')['sex']);
    }

    /**
     * @return array<string, array{string, string}>
     *
     * @codeCoverageIgnore Data providers run before coverage instrumentation starts.
     */
    public static function sexProvider(): array
    {
        return [
            'Male' => ['Male', 'M'],
            'Female' => ['Female', 'F'],
            'lowercase m' => ['m', 'M'],
            'Other' => ['Other', 'O'],
            'blank -> U' => ['', 'U'],
            'garbage -> U' => ['xyz', 'U'],
        ];
    }

    public function testEmptyDobBecomesEmptyString(): void
    {
        $row = self::openEmrRow();
        $row['DOB'] = '';
        self::assertSame('', PatientMapper::toCanonical($row, 'create')['birthDate']);
    }

    public function testMissingKeysDoNotError(): void
    {
        $c = PatientMapper::toCanonical(['pubpid' => '9'], 'create');
        self::assertSame('9', $c['mrn']);
        self::assertSame('', $c['familyName']);
        self::assertSame('U', $c['sex']);
    }

    public function testToOpenEmrIsInverseOfToCanonical(): void
    {
        $c = PatientMapper::toCanonical(self::openEmrRow(), 'create');
        $back = PatientMapper::toOpenEmr($c);

        self::assertSame('12345', $back['pubpid']);
        self::assertSame('Doe', $back['lname']);
        self::assertSame('Jane', $back['fname']);
        self::assertSame('Q', $back['mname']);
        self::assertSame('1980-01-15', $back['DOB'], 'HL7 date should round-trip to Y-m-d');
        self::assertSame('Female', $back['sex'], 'HL7 F should round-trip to Female');
        self::assertSame('Austin', $back['city']);
        self::assertSame('78701', $back['postal_code']);
        self::assertSame('555-1234', $back['phone_home']);
    }

    public function testToOpenEmrHandlesShortOrMissingDate(): void
    {
        $back = PatientMapper::toOpenEmr(['mrn' => '1', 'birthDate' => '', 'sex' => 'M']);
        self::assertSame('', $back['DOB']);
        self::assertSame('Male', $back['sex']);
    }
}
