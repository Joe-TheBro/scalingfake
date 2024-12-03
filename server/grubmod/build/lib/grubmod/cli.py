import click
import os
import shutil
import sys


@click.command()
@click.option("--kernel-version", required=True, help="Kernel version to target")
def modify_grub(kernel_version):
    print(f"Modifying GRUB for kernel version: {kernel_version}")

    grub_config = "/boot/grub/grub.cfg"
    temp_file = "/boot/grub/grub.cfg.temp"

    if not os.path.exists(grub_config):
        print("Error: GRUB configuration file not found.")
        sys.exit(1)

    if not os.access(grub_config, os.W_OK):
        print("Error: Permission denied. Please run this script as root.")
        sys.exit(1)

    output = []
    in_section = False

    try:
        with open(grub_config, "r") as f:
            lines = f.readlines()
            iterator = iter(lines)

            for line in iterator:
                section_start = f"menuentry 'Ubuntu, with Linux {kernel_version}'"
                if section_start in line:
                    print("DEBUG: Found kernel section start.")
                    in_section = True
                    output.append(line)
                    continue

                if in_section and line.strip() == "}":
                    print("DEBUG: End of kernel section.")
                    in_section = False
                    output.append(line)
                    continue

                if in_section and line.strip() == 'if [ "${initrdfail}" = 1 ]; then':
                    print("DEBUG: Skipping lines in initrdfail block.")
                    try:
                        for _ in range(1):  # Skip two lines
                            next(iterator)
                        output.append(next(iterator))  # Include next line
                        next(iterator)  # Skip line
                        output.append(next(iterator))  # Include next line
                        for _ in range(5):  # Skip five lines
                            next(iterator)
                    except StopIteration:
                        print(
                            "DEBUG: Reached end of file unexpectedly while skipping lines."
                        )
                        break
                    continue
                output.append(line)

    except Exception as e:
        print(f"Error while reading GRUB config: {e}")
        sys.exit(1)

    try:
        with open(temp_file, "w") as f:
            f.writelines(output)

        shutil.move(temp_file, grub_config)
        print("GRUB modified successfully.")
    except Exception as e:
        print(f"Error writing GRUB config: {e}")
        if os.path.exists(temp_file):
            os.remove(temp_file)
        sys.exit(1)


if __name__ == "__main__":
    modify_grub()
