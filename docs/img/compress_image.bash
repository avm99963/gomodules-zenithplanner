#!/bin/bash
# Helper tool used to compress screenshots from PNG to JPEG
magick -quality 85 $1 $2
