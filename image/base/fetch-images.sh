IMAGEDIR=~/ranchervm-images/
cat imagelist.txt | grep -v "^#" | while read ENTRY; do
	ALIAS=$(echo $ENTRY | cut -d' ' -f1)
	URL=$(echo $ENTRY | cut -d' ' -f2)
	FILENAME=$(basename "$URL")
	echo filename $FILENAME
	echo url $URL
	if [ -z "$ALIAS" ] || [ -z "$URL" ] || [ -z "$FILENAME" ]; then
		echo "config file entry is incorrect: $ENTRY"
		exit 1
	fi

	wget "$URL" -O $IMAGEDIR/$FILENAME && \
	ln -s $FILENAME $IMAGEDIR/$ALIAS || \
	echo "download failed for url $URL"
done
