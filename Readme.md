# Simple http/https proxy

To start a server first run gen_ca.sh, trust it within your system (or add your cert to the root), and then run

``docker compose up -d``

Each param from param.txt is being added to a request, so it may be quite slow.