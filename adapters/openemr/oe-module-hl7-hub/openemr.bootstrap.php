<?php

/**
 * Bootstrap for the OpenEMR HL7 Hub adapter module.
 *
 * Bridges OpenEMR to the EMR-agnostic HL7 hub: it maps OpenEMR patient events
 * to the hub's canonical model and forwards them, and exposes a signed webhook
 * for the hub to write canonical data back via the service layer.
 *
 * @package   OpenEMR
 * @link      https://www.open-emr.org
 * @author    Chris Dickman
 * @copyright Copyright (c) 2026 Chris Dickman
 * @license   https://github.com/openemr/openemr/blob/master/LICENSE GNU General Public License 3
 */

declare(strict_types=1);

use OpenEMR\Core\ModulesClassLoader;
use OpenEMR\Modules\Hl7Hub\Bootstrap;
use Symfony\Component\EventDispatcher\EventDispatcherInterface;

/**
 * @var ModulesClassLoader $classLoader Injected by ModulesApplication::loadCustomModule
 */
$classLoader->registerNamespaceIfNotExists(
    'OpenEMR\\Modules\\Hl7Hub\\',
    __DIR__ . DIRECTORY_SEPARATOR . 'src'
);

/**
 * @var EventDispatcherInterface $eventDispatcher Injected by the module loader
 */
if (!empty($eventDispatcher) && $eventDispatcher instanceof EventDispatcherInterface) {
    (new Bootstrap($eventDispatcher))->subscribeToEvents();
}
