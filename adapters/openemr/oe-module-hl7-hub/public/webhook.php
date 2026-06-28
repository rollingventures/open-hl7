<?php

/**
 * Inbound webhook: the HL7 hub POSTs a canonical patient here (decoded from an
 * inbound ADT) and this endpoint writes it into OpenEMR via the service layer.
 *
 * Secured by the shared secret in the X-Hub-Secret header (constant-time
 * compared). M1 performs a create; upsert-by-MRN matching is a follow-up.
 *
 * @package   OpenEMR
 * @link      https://www.open-emr.org
 * @author    Chris Dickman
 * @copyright Copyright (c) 2026 Chris Dickman
 * @license   https://github.com/openemr/openemr/blob/master/LICENSE GNU General Public License 3
 */

declare(strict_types=1);

// Public endpoint: no interactive OpenEMR auth; authenticated by shared secret.
$ignoreAuth = true;
require_once __DIR__ . "/../../../../globals.php";

use OpenEMR\Common\Logging\SystemLogger;
use OpenEMR\Modules\Hl7Hub\Canonical\PatientMapper;
use OpenEMR\Modules\Hl7Hub\GlobalConfig;
use OpenEMR\Services\PatientService;

header('Content-Type: application/json');

$logger = new SystemLogger();
$config = GlobalConfig::fromEnvironment();

/**
 * @param array<string, mixed> $payload
 */
function respond(int $status, array $payload): never
{
    http_response_code($status);
    echo json_encode($payload, JSON_THROW_ON_ERROR);
    exit;
}

if ($_SERVER['REQUEST_METHOD'] !== 'POST') {
    respond(405, ['error' => 'method not allowed']);
}

// Shared-secret auth (constant-time).
$provided = (string) ($_SERVER['HTTP_X_HUB_SECRET'] ?? '');
if ($config->secret === '' || !hash_equals($config->secret, $provided)) {
    respond(401, ['error' => 'unauthorized']);
}

$raw = file_get_contents('php://input');
if ($raw === false || $raw === '') {
    respond(400, ['error' => 'empty body']);
}

try {
    /** @var array<string, mixed> $canonical */
    $canonical = json_decode($raw, true, 512, JSON_THROW_ON_ERROR);
} catch (\Throwable $e) {
    $logger->warning('HL7 hub webhook: invalid JSON', ['exception' => $e]);
    respond(400, ['error' => 'invalid json']);
}

try {
    $data = PatientMapper::toOpenEmr($canonical);
    $service = new PatientService();
    $result = $service->insert($data);

    if ($result->hasErrors()) {
        $logger->error('HL7 hub webhook: patient insert failed', [
            'errors' => $result->getValidationMessages(),
        ]);
        respond(422, ['error' => 'patient could not be saved']);
    }

    respond(201, ['status' => 'created']);
} catch (\Throwable $e) {
    $logger->error('HL7 hub webhook: unexpected error', ['exception' => $e]);
    respond(500, ['error' => 'internal error']);
}
