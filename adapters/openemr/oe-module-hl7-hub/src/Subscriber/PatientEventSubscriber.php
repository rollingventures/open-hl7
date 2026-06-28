<?php

/**
 * @package   OpenEMR
 * @link      https://www.open-emr.org
 * @author    Chris Dickman
 * @copyright Copyright (c) 2026 Chris Dickman
 * @license   https://github.com/openemr/openemr/blob/master/LICENSE GNU General Public License 3
 */

declare(strict_types=1);

namespace OpenEMR\Modules\Hl7Hub\Subscriber;

use OpenEMR\Events\Patient\PatientCreatedEvent;
use OpenEMR\Events\Patient\PatientUpdatedEvent;
use OpenEMR\Modules\Hl7Hub\Canonical\PatientMapper;
use OpenEMR\Modules\Hl7Hub\Client\HubClient;
use Psr\Log\LoggerInterface;
use Symfony\Component\EventDispatcher\EventSubscriberInterface;
use Throwable;

/**
 * Forwards OpenEMR patient create/update events to the hub as canonical ADT
 * feeds. A hub outage must never block a clinical save, so dispatch failures
 * are logged and swallowed here (the hub-side store/retry owns delivery).
 */
final class PatientEventSubscriber implements EventSubscriberInterface
{
    public function __construct(
        private readonly HubClient $client,
        private readonly LoggerInterface $logger,
    ) {
    }

    /**
     * @return array<string, string>
     */
    public static function getSubscribedEvents(): array
    {
        return [
            PatientCreatedEvent::EVENT_HANDLE => 'onPatientCreated',
            PatientUpdatedEvent::EVENT_HANDLE => 'onPatientUpdated',
        ];
    }

    public function onPatientCreated(PatientCreatedEvent $event): void
    {
        $this->dispatch($event->getPatientData(), 'create');
    }

    public function onPatientUpdated(PatientUpdatedEvent $event): void
    {
        $this->dispatch($event->getNewPatientData(), 'update');
    }

    /**
     * @param array<string, mixed> $patientData
     */
    private function dispatch(array $patientData, string $event): void
    {
        try {
            $this->client->sendPatient(PatientMapper::toCanonical($patientData, $event));
        } catch (Throwable $e) {
            $this->logger->error('Failed to forward patient event to HL7 hub', [
                'event' => $event,
                'exception' => $e,
            ]);
        }
    }
}
