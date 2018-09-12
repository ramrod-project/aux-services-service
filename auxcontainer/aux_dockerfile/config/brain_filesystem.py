from brain.binary.filesystem import start_filesystem, BrainStoreConfig
from sys import argv, stderr
from os import mkdir
from os import environ

if __name__ == "__main__":
    environ["STAGE"] = environ.get("STAGE", "PROD")  # probably always prod
    if len(argv) < 2:
        stderr.write("Usage: \n\t{} <mountpoint>\n".format(argv[0]))
        exit(1)
    mountpoint = argv[1]
    try:
        mkdir(argv[1])
    except FileExistsError:
        pass
    bsc = BrainStoreConfig(allow_remove=True, allow_list=True)
    start_filesystem(argv[1], bsc)
