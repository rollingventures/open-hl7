<?php

/**
 * @package   OpenEMR
 * @link      https://www.open-emr.org
 * @author    Chris Dickman
 * @copyright Copyright (c) 2026 Chris Dickman
 * @license   https://github.com/openemr/openemr/blob/master/LICENSE GNU General Public License 3
 */

declare(strict_types=1);

namespace OpenEMR\Modules\Hl7Hub\Canonical;

/**
 * Maps OpenEMR's `patient_data` row shape to/from the hub's neutral canonical
 * patient JSON. This is the ONLY place that knows OpenEMR's column names — keep
 * all EMR-specific field knowledge here so the rest of the module stays generic.
 */
final class PatientMapper
{
    /**
     * @param array<string, mixed> $p   OpenEMR patient_data row
     * @param string               $event "create" | "update"
     *
     * @return array<string, mixed> canonical patient
     */
    public static function toCanonical(array $p, string $event): array
    {
        return [
            'mrn' => self::str($p['pubpid'] ?? '') ?: self::str($p['pid'] ?? ''),
            'familyName' => self::str($p['lname'] ?? ''),
            'givenName' => self::str($p['fname'] ?? ''),
            'middleName' => self::str($p['mname'] ?? ''),
            'birthDate' => self::toHl7Date(self::str($p['DOB'] ?? '')),
            'sex' => self::toHl7Sex(self::str($p['sex'] ?? '')),
            'address' => [
                'line1' => self::str($p['street'] ?? ''),
                'city' => self::str($p['city'] ?? ''),
                'state' => self::str($p['state'] ?? ''),
                'zip' => self::str($p['postal_code'] ?? ''),
                'country' => self::str($p['country_code'] ?? ''),
            ],
            'phone' => self::str($p['phone_home'] ?? ''),
            'event' => $event,
            'source' => ['emr' => 'openemr', 'facility' => 'OPENEMR'],
        ];
    }

    /**
     * Inverse: canonical patient JSON -> OpenEMR patient_data array for the
     * service layer.
     *
     * @param array<string, mixed> $c canonical patient
     *
     * @return array<string, string>
     */
    public static function toOpenEmr(array $c): array
    {
        $addr = is_array($c['address'] ?? null) ? $c['address'] : [];

        return [
            'pubpid' => self::str($c['mrn'] ?? ''),
            'lname' => self::str($c['familyName'] ?? ''),
            'fname' => self::str($c['givenName'] ?? ''),
            'mname' => self::str($c['middleName'] ?? ''),
            'DOB' => self::fromHl7Date(self::str($c['birthDate'] ?? '')),
            'sex' => self::fromHl7Sex(self::str($c['sex'] ?? '')),
            'street' => self::str($addr['line1'] ?? ''),
            'city' => self::str($addr['city'] ?? ''),
            'state' => self::str($addr['state'] ?? ''),
            'postal_code' => self::str($addr['zip'] ?? ''),
            'country_code' => self::str($addr['country'] ?? ''),
            'phone_home' => self::str($c['phone'] ?? ''),
        ];
    }

    private static function str(mixed $v): string
    {
        return is_scalar($v) ? (string) $v : '';
    }

    /** "1980-01-15" -> "19800115" (HL7 TS date). */
    private static function toHl7Date(string $dob): string
    {
        return preg_replace('/[^0-9]/', '', $dob) ?? '';
    }

    /** "19800115" -> "1980-01-15". */
    private static function fromHl7Date(string $hl7): string
    {
        if (strlen($hl7) < 8) {
            return '';
        }
        return substr($hl7, 0, 4) . '-' . substr($hl7, 4, 2) . '-' . substr($hl7, 6, 2);
    }

    private static function toHl7Sex(string $sex): string
    {
        return match (strtolower($sex)) {
            'male', 'm' => 'M',
            'female', 'f' => 'F',
            'other', 'o' => 'O',
            default => 'U',
        };
    }

    private static function fromHl7Sex(string $sex): string
    {
        return match (strtoupper($sex)) {
            'M' => 'Male',
            'F' => 'Female',
            'O' => 'Other',
            default => 'Unknown',
        };
    }
}
