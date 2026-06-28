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

use OpenEMR\Common\Logging\SystemLogger;
use OpenEMR\Modules\Hl7Hub\Client\HubClient;
use OpenEMR\Modules\Hl7Hub\Subscriber\PatientEventSubscriber;
use Symfony\Component\EventDispatcher\EventDispatcherInterface;

/**
 * Wires the module's event subscribers. Kept intentionally thin: all real work
 * lives in the subscriber, mapper and client so each is unit-testable.
 */
final class Bootstrap
{
    public function __construct(
        private readonly EventDispatcherInterface $eventDispatcher
    ) {
    }

    public function subscribeToEvents(): void
    {
        $config = GlobalConfig::fromEnvironment();

        // No hub URL configured -> module is inert (no listeners registered).
        if (!$config->isEnabled()) {
            return;
        }

        $logger = new SystemLogger();
        $client = new HubClient($config, $logger);

        $this->eventDispatcher->addSubscriber(
            new PatientEventSubscriber($client, $logger)
        );
    }
}
