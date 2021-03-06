==============
mixer.add-rpms
==============

---------------------
Add local RPMs to mix
---------------------

:Copyright: \(C) 2018 Intel Corporation, CC-BY-SA-3.0
:Manual section: 1


SYNOPSIS
========

``mixer add-rpms [flags]``


DESCRIPTION
===========

Adds RPMs from the `LOCAL_RPM_DIR` (configured in the `builder.conf`) to the
local RPM repository to be used in creating a mix.


OPTIONS
=======

In addition to the globally recognized ``mixer`` flags (see ``mixer``\(1) for
more details), the following options are recognized.

-  ``-c, --config {path}``

   Optionally tell ``mixer`` to use the configuration file at `path`. Uses the
   default `builder.conf` in the mixer workspace if this option is not provided.

-  ``-h, --help``

   Display ``add-rpm`` help information and exit.


EXIT STATUS
===========

On success, 0 is returned. A non-zero return code indicates a failure.

SEE ALSO
--------

* ``mixer``\(1)
