"""CloudLab profile: single bare-metal build server for monolift.

The repo is cloned to /local/repository on the node automatically.
Run cloudlab/setup.sh at boot to install Go, Docker, and build deps.
"""

import geni.portal as portal
import geni.rspec.pg as pg

pc = portal.Context()

pc.defineParameter("hardware_type", "Hardware type",
                   portal.ParameterType.STRING, "c6525-25g",
                   [("c6525-25g", "c6525-25g (16-core AMD 7302P, 128GB, Utah)"),
                    ("c6525-100g", "c6525-100g (24-core AMD 7402P, 128GB, Utah)"),
                    ("xl170", "xl170 (10-core Xeon E5-2640v4, 64GB, Utah)"),
                    ("c220g5", "c220g5 (20-core Xeon Silver 4114, 192GB, Wisconsin)"),
                    ("c6420", "c6420 (32-core Xeon Gold 6142, 384GB, Clemson)"),
                    ("c6220", "c6220 (16-core Xeon E5-2650v2, 64GB, Apt)")],
                   longDescription="Pick based on availability. c6525-25g is a good default: "
                   "16 cores, 128GB RAM, dual 480GB SSDs.")

pc.defineParameter("os_image", "Disk image",
                   portal.ParameterType.STRING,
                   "urn:publicid:IDN+emulab.net+image+emulab-ops:UBUNTU22-64-STD",
                   [("urn:publicid:IDN+emulab.net+image+emulab-ops:UBUNTU22-64-STD", "Ubuntu 22.04"),
                    ("urn:publicid:IDN+emulab.net+image+emulab-ops:UBUNTU24-64-STD", "Ubuntu 24.04")])

params = pc.bindParameters()

request = pc.makeRequestRSpec()

node = request.RawPC("build")
node.hardware_type = params.hardware_type
node.disk_image = params.os_image
node.addService(pg.Execute(shell="bash", command="sudo /local/repository/cloudlab/setup.sh"))

pc.printRequestRSpec(request)
