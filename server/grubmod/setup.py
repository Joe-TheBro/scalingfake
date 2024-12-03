from setuptools import setup, find_packages

setup(
    name='grubmod',
    version='0.9.1',
    packages=find_packages(),
    install_requires=[
        'click',
    ],
    entry_points='''
        [console_scripts]
        grubmod=grubmod.cli:modify_grub
    ''',
)