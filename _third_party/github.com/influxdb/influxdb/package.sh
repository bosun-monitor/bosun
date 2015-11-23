#!/usr/bin/env bash

###########################################################################
# Packaging script which creates debian and RPM packages. It optionally
# tags the repo with the given version.
#
# Requirements: GOPATH must be set. 'fpm' must be on the path, and the AWS
# CLI tools must also be installed.
#
#    https://github.com/jordansissel/fpm
#    http://aws.amazon.com/cli/
#
# Packaging process: to package a build, simple execute:
#
#    package.sh <version>
#
# where <version> is the desired version. If generation of a debian and RPM
# package is successful, the script will offer to tag the repo using the
# supplied version string.
#
# See package.sh -h for options
#
# AWS upload: the script will also offer to upload the packages to S3. If
# this option is selected, the credentials should be present in the file
# ~/aws.conf. The contents should be of the form:
#
#    [default]
#    aws_access_key_id=<access ID>
#    aws_secret_access_key=<secret key>
#    region = us-east-1
#
# Trim the leading spaces when creating the file. The script will exit if
# S3 upload is requested, but this file does not exist.

[ -z $DEBUG ] || set -x

AWS_FILE=~/aws.conf

INSTALL_ROOT_DIR=/opt/influxdb
INFLUXDB_LOG_DIR=/var/log/influxdb
INFLUXDB_DATA_DIR=/var/opt/influxdb
CONFIG_ROOT_DIR=/etc/opt/influxdb
LOGROTATE_DIR=/etc/logrotate.d

SAMPLE_CONFIGURATION=etc/config.sample.toml
INITD_SCRIPT=scripts/init.sh
SYSTEMD_SCRIPT=scripts/influxdb.service
LOGROTATE=scripts/logrotate

TMP_WORK_DIR=`mktemp -d`
POST_INSTALL_PATH=`mktemp`
POST_UNINSTALL_PATH=`mktemp`
ARCH=`uname -i`
LICENSE=MIT
URL=influxdb.com
MAINTAINER=support@influxdb.com
VENDOR=Influxdb
DESCRIPTION="Distributed time-series database"

# Allow path to FPM to be set by environment variables. Some execution contexts
# like cron don't have PATH set correctly to pick it up.
if [ -z "$FPM" ]; then
    FPM=`which fpm`
fi

GO_VERSION="go1.5"
GOPATH_INSTALL=
BINS=(
    influxd
    influx
    )

###########################################################################
# Helper functions.

# usage prints simple usage information.
usage() {
    cat << EOF >&2
$0 [-h] [-p|-w] [-t <dist>] [-r <number>] <version>

    <version> should be a dotted version such as 0.9.5.

    -r release candidate number, if any.
       Example: -r 7
    -p just build packages
    -w build packages for current working directory
       imply -p
    -t <dist>
       build package for <dist>
       <dist> can be rpm, tar or deb
       can have multiple -t

    Examples:

        $0 0.9.5 -r 9 # Creates 0.9.5-rc9
        $0 0.9.4      # Creates 0.9.4

EOF
    cleanup_exit $1
}

# full_version echoes the full version string, given a version and an optiona;l
# RC number. If the just the version is present, that is echoed. If the RC is
# also provided, then "rc" and the number is concatenated with the version.
# For example, 0.9.4rc4 would be returned if version was 0.9.4 and the RC number
# was 4.
full_version() {
    version=$1
    rc=$2
    if [ -z "$rc" ]; then
        echo $version
    else
        echo ${version}-rc${rc}
    fi
}

# rpm_release echoes the RPM release or "iteration" given an RC number.
rpm_release() {
    rc=$1
    if [ -z "$rc" ]; then
        echo 1
    else
        echo 0.1.rc${rc}
    fi
}

# cleanup_exit removes all resources created during the process and exits with
# the supplied returned code.
cleanup_exit() {
    rm -r $TMP_WORK_DIR
    rm $POST_INSTALL_PATH
    rm $POST_UNINSTALL_PATH
    exit $1
}

# current_branch echos the current git branch.
current_branch() {
    echo `git rev-parse --abbrev-ref HEAD`
}

# check_gopath sanity checks the value of the GOPATH env variable, and determines
# the path where build artifacts are installed. GOPATH may be a colon-delimited
# list of directories.
check_gopath() {
    [ -z "$GOPATH" ] && echo "GOPATH is not set." && cleanup_exit 1
    GOPATH_INSTALL=`echo $GOPATH | cut -d ':' -f 1`
    [ ! -d "$GOPATH_INSTALL" ] && echo "GOPATH_INSTALL is not a directory." && cleanup_exit 1
    echo "GOPATH ($GOPATH) looks sane, using $GOPATH_INSTALL for installation."
}

check_gvm() {
    if [ -n "$GOPATH" ]; then
        existing_gopath=$GOPATH
    fi

    source $HOME/.gvm/scripts/gvm
    which gvm
    if [ $? -ne 0 ]; then
        echo "gvm not found -- aborting."
        cleanup_exit $1
    fi
    gvm use $GO_VERSION
    if [ $? -ne 0 ]; then
        echo "gvm cannot find Go version $GO_VERSION -- aborting."
        cleanup_exit $1
    fi

    # Keep any existing GOPATH set.
    if [ -n "$existing_gopath" ]; then
        GOPATH=$existing_gopath
    fi
}

# check_clean_tree ensures that no source file is locally modified.
check_clean_tree() {
    modified=$(git ls-files --modified | wc -l)
    if [ $modified -ne 0 ]; then
        echo "The source tree is not clean -- aborting."
        cleanup_exit 1
    fi
    echo "Git tree is clean."
}

# update_tree ensures the tree is in-sync with the repo.
update_tree() {
    git pull origin $TARGET_BRANCH
    if [ $? -ne 0 ]; then
        echo "Failed to pull latest code -- aborting."
        cleanup_exit 1
    fi
    git fetch --tags
    if [ $? -ne 0 ]; then
        echo "Failed to fetch tags -- aborting."
        cleanup_exit 1
    fi
    echo "Git tree updated successfully."
}

# check_tag_exists checks if the existing release already exists in the tags.
check_tag_exists () {
    version=$1
    git tag | grep -q "^v$version$"
    if [ $? -eq 0 ]; then
        echo "Proposed version $version already exists as a tag -- aborting."
        cleanup_exit 1
    fi
}

# make_dir_tree creates the directory structure within the packages.
make_dir_tree() {
    work_dir=$1
    version=$2
    mkdir -p $work_dir/$INSTALL_ROOT_DIR/versions/$version/scripts
    if [ $? -ne 0 ]; then
        echo "Failed to create installation directory -- aborting."
        cleanup_exit 1
    fi
    mkdir -p $work_dir/$CONFIG_ROOT_DIR
    if [ $? -ne 0 ]; then
        echo "Failed to create configuration directory -- aborting."
        cleanup_exit 1
    fi
    mkdir -p $work_dir/$LOGROTATE_DIR
    if [ $? -ne 0 ]; then
        echo "Failed to create logrotate directory -- aborting."
        cleanup_exit 1
    fi
}

# do_build builds the code. The version and commit must be passed in.
do_build() {
    for b in ${BINS[*]}; do
        rm -f $GOPATH_INSTALL/bin/$b
    done

    if [ -n "$WORKING_DIR" ]; then
        STASH=`git stash create -a`
        if [ $? -ne 0 ]; then
            echo "WARNING: failed to stash uncommited local changes"
        fi
        git reset --hard
    fi

    go get -u -f -d ./...
    if [ $? -ne 0 ]; then
        echo "WARNING: failed to 'go get' packages."
    fi

    git checkout $TARGET_BRANCH # go get switches to master, so ensure we're back.

    if [ -n "$WORKING_DIR" ]; then
        git stash apply $STASH
        if [ $? -ne 0 ]; then #and apply previous uncommited local changes
            echo "WARNING: failed to restore uncommited local changes"
        fi
    fi

    version=$1
    commit=`git rev-parse HEAD`
    branch=`current_branch`
    if [ $? -ne 0 ]; then
        echo "Unable to retrieve current commit -- aborting"
        cleanup_exit 1
    fi

    date=`date -u --iso-8601=seconds`
    go install -a -ldflags="-X main.version=$version -X main.branch=$branch -X main.commit=$commit -X main.buildTime='$date'" ./...
    if [ $? -ne 0 ]; then
        echo "Build failed, unable to create package -- aborting"
        cleanup_exit 1
    fi
    echo "Build completed successfully."
}

# generate_postinstall_script creates the post-install script for the
# package. It must be passed the version.
generate_postinstall_script() {
    version=$1
    cat  <<EOF >$POST_INSTALL_PATH
#!/bin/sh
rm -f $INSTALL_ROOT_DIR/influxd
rm -f $INSTALL_ROOT_DIR/influx
rm -f $INSTALL_ROOT_DIR/init.sh
ln -s $INSTALL_ROOT_DIR/versions/$version/influxd $INSTALL_ROOT_DIR/influxd
ln -s $INSTALL_ROOT_DIR/versions/$version/influx $INSTALL_ROOT_DIR/influx
ln -s $INSTALL_ROOT_DIR/versions/$version/scripts/init.sh $INSTALL_ROOT_DIR/init.sh

if ! id influxdb >/dev/null 2>&1; then
        useradd --system -U -M influxdb
fi

# Systemd
if which systemctl > /dev/null 2>&1 ; then
    cp $INSTALL_ROOT_DIR/versions/$version/scripts/influxdb.service \
        /lib/systemd/system/influxdb.service
    systemctl enable influxdb

# Sysv
else
    rm -f /etc/init.d/influxdb
    ln -sfn $INSTALL_ROOT_DIR/init.sh /etc/init.d/influxdb
    chmod +x /etc/init.d/influxdb
    if which update-rc.d > /dev/null 2>&1 ; then
        update-rc.d -f influxdb remove
        update-rc.d influxdb defaults
    else
        chkconfig --add influxdb
    fi
fi

chown -R -L influxdb:influxdb $INSTALL_ROOT_DIR
chmod -R a+rX $INSTALL_ROOT_DIR

mkdir -p $INFLUXDB_LOG_DIR
chown -R -L influxdb:influxdb $INFLUXDB_LOG_DIR
mkdir -p $INFLUXDB_DATA_DIR
chown -R -L influxdb:influxdb $INFLUXDB_DATA_DIR
EOF
    echo "Post-install script created successfully at $POST_INSTALL_PATH"
}

generate_postuninstall_script() {
    cat << EOF >$POST_UNINSTALL_PATH
#!/bin/sh
if [ -d $INSTALL_ROOT_DIR ]; then
    rm -r "$INSTALL_ROOT_DIR"
fi

EOF
    echo "Post-uninstall script created successfully at $POST_UNINSTALL_PATH"
}

###########################################################################
# Process options
while :
do
  case $1 in
    -h | --help)
	    usage 0
	    ;;

    -p | --packages-only)
	    PACKAGES_ONLY="PACKAGES_ONLY"
	    shift
	    ;;

    -t | --target)
        case "$2" in
            'tar') TAR_WANTED="gz"
                 ;;
            'deb') DEB_WANTED="deb"
                 ;;
            'rpm') RPM_WANTED="rpm"
                 ;;
            *)
                 echo "Unknown target distribution $2"
                 usage 1
                 ;;
        esac
        shift 2
        ;;

    -r)
        RC=$2
        if [ -z "$RC" ]; then
            echo "RC number required"
        fi
        shift 2
        ;;

    -w | --working-directory)
	PACKAGES_ONLY="PACKAGES_ONLY"
        WORKING_DIR="WORKING_DIR"
	    shift
	    ;;

    -*)
        echo "Unknown option $1"
        usage 1
        ;;

    ?*)
        if [ -z $VERSION ]; then
           VERSION=$1
           shift
        else
           echo "$1 : aborting version already set to $VERSION"
           usage 1
        fi

        echo $VERSION | grep -i '[r|c]' 2>&1 >/dev/null
        if [ $? -ne 1 -a -z "$NIGHTLY_BUILD" ]; then
            echo
            echo "$VERSION contains reference to RC - specify RC separately"
            echo
            usage 1
        fi
        ;;

     *) break
  esac
done

if [ -z "$DEB_WANTED$RPM_WANTED$TAR_WANTED" ]; then
  TAR_WANTED="gz"
  DEB_WANTED="deb"
  RPM_WANTED="rpm"
fi

if [ -z "$VERSION" ]; then
  echo -e "Missing version"
  usage 1
fi

###########################################################################
# Start the packaging process.

echo -e "\nStarting package process...\n"

# Ensure the current is correct.
TARGET_BRANCH=`current_branch`
if [ -z "$NIGHTLY_BUILD" -a -z "$PACKAGES_ONLY" ]; then
echo -n "Current branch is $TARGET_BRANCH. Start packaging this branch? [Y/n] "
    read response
    response=`echo $response | tr 'A-Z' 'a-z'`
    if [ "x$response" == "xn" ]; then
        echo "Packaging aborted."
        cleanup_exit 1
    fi
fi

check_gvm
check_gopath
if [ -z "$NIGHTLY_BUILD" -a -z "$PACKAGES_ONLY" ]; then
       check_clean_tree
       update_tree
       check_tag_exists `full_version $VERSION $RC`
fi

do_build `full_version $VERSION $RC`
make_dir_tree $TMP_WORK_DIR `full_version $VERSION $RC`

###########################################################################
# Copy the assets to the installation directories.

for b in ${BINS[*]}; do
    cp $GOPATH_INSTALL/bin/$b $TMP_WORK_DIR/$INSTALL_ROOT_DIR/versions/`full_version $VERSION $RC`
    if [ $? -ne 0 ]; then
        echo "Failed to copy binaries to packaging directory -- aborting."
        cleanup_exit 1
    fi
done
echo "${BINS[*]} copied to $TMP_WORK_DIR/$INSTALL_ROOT_DIR/versions/`full_version $VERSION $RC`"

cp $INITD_SCRIPT $TMP_WORK_DIR/$INSTALL_ROOT_DIR/versions/`full_version $VERSION $RC`/scripts
if [ $? -ne 0 ]; then
    echo "Failed to copy init.d script to packaging directory -- aborting."
    cleanup_exit 1
fi
echo "$INITD_SCRIPT copied to $TMP_WORK_DIR/$INSTALL_ROOT_DIR/versions/`full_version $VERSION $RC`/scripts"

cp $SYSTEMD_SCRIPT $TMP_WORK_DIR/$INSTALL_ROOT_DIR/versions/`full_version $VERSION $RC`/scripts
if [ $? -ne 0 ]; then
    echo "Failed to copy systemd script to packaging directory -- aborting."
    cleanup_exit 1
fi
echo "$SYSTEMD_SCRIPT copied to $TMP_WORK_DIR/$INSTALL_ROOT_DIR/versions/`full_version $VERSION $RC`/scripts"

cp $SAMPLE_CONFIGURATION $TMP_WORK_DIR/$CONFIG_ROOT_DIR/influxdb.conf
if [ $? -ne 0 ]; then
    echo "Failed to copy $SAMPLE_CONFIGURATION to packaging directory -- aborting."
    cleanup_exit 1
fi

cp $LOGROTATE $TMP_WORK_DIR/$LOGROTATE_DIR/influxd
if [ $? -ne 0 ]; then
    echo "Failed to copy logrotate configuration to packaging directory -- aborting."
    cleanup_exit 1
fi

generate_postinstall_script `full_version $VERSION $RC`
generate_postuninstall_script

###########################################################################
# Create the actual packages.

if [ -z "$NIGHTLY_BUILD" -a -z "$PACKAGES_ONLY" ]; then
    echo -n "Commence creation of $ARCH packages, version `full_version $VERSION $RC`? [Y/n] "
    read response
    response=`echo $response | tr 'A-Z' 'a-z'`
    if [ "x$response" == "xn" ]; then
        echo "Packaging aborted."
        cleanup_exit 1
    fi
fi

if [ $ARCH == "i386" ]; then
    rpm_package=influxdb-${VERSION}-1.i686.rpm # RPM packages use 1 for default package release.
    debian_package=influxdb_`full_version $VERSION $RC`_i686.deb
    deb_args="-a i686"
    rpm_args="setarch i686"
elif [ $ARCH == "arm" ]; then
    rpm_package=influxdb-${VERSION}-1.armel.rpm
    debian_package=influxdb_`full_version $VERSION $RC`_armel.deb
else
    rpm_package=influxdb-${VERSION}-1.x86_64.rpm
    debian_package=influxdb_`full_version $VERSION $RC`_amd64.deb
fi

COMMON_FPM_ARGS="\
--log error \
-C $TMP_WORK_DIR \
--vendor $VENDOR \
--url $URL \
--license $LICENSE \
--maintainer $MAINTAINER \
--after-install $POST_INSTALL_PATH \
--after-remove $POST_UNINSTALL_PATH \
--name influxdb \
--description "$DESCRIPTION" \
--config-files $CONFIG_ROOT_DIR \
--config-files $LOGROTATE_DIR"

if [ -n "$DEB_WANTED" ]; then
    $FPM -s dir -t deb $deb_args $COMMON_FPM_ARGS --version `full_version $VERSION $RC` .
    if [ $? -ne 0 ]; then
        echo "Failed to create Debian package -- aborting."
        cleanup_exit 1
    fi
    echo "Debian package created successfully."
fi

if [ -n "$TAR_WANTED" ]; then
    $FPM -s dir -t tar --prefix influxdb_`full_version $VERSION $RC`_${ARCH} -p influxdb_`full_version $VERSION $RC`_${ARCH}.tar.gz $COMMON_FPM_ARGS --version `full_version $VERSION $RC ` .
    if [ $? -ne 0 ]; then
        echo "Failed to create Tar package -- aborting."
        cleanup_exit 1
    fi
    echo "Tar package created successfully."
fi

if [ -n "$RPM_WANTED" ]; then
    $rpm_args $FPM -s dir -t rpm $COMMON_FPM_ARGS --depends coreutils --version $VERSION --iteration `rpm_release $RC` .
    if [ $? -ne 0 ]; then
        echo "Failed to create RPM package -- aborting."
        cleanup_exit 1
    fi
    echo "RPM package created successfully."
fi

###########################################################################
# Offer to tag the repo.

if [ -z "$NIGHTLY_BUILD" -a -z "$PACKAGES_ONLY" ]; then
    echo -n "Tag source tree with v`full_version $VERSION $RC` and push to repo? [y/N] "
    read response
    response=`echo $response | tr 'A-Z' 'a-z'`
    if [ "x$response" == "xy" ]; then
        echo "Creating tag v`full_version $VERSION $RC` and pushing to repo"
        git tag v`full_version $VERSION $RC`
        if [ $? -ne 0 ]; then
            echo "Failed to create tag v`full_version $VERSION $RC` -- aborting"
            cleanup_exit 1
        fi
        echo "Tag v`full_version $VERSION $RC` created"
        git push origin v`full_version $VERSION $RC`
        if [ $? -ne 0 ]; then
            echo "Failed to push tag v`full_version $VERSION $RC` to repo -- aborting"
            cleanup_exit 1
        fi
        echo "Tag v`full_version $VERSION $RC` pushed to repo"
    else
        echo "Not creating tag v`full_version $VERSION $RC`."
    fi
fi

###########################################################################
# Offer to publish the packages.

if [ -z "$NIGHTLY_BUILD" -a -z "$PACKAGES_ONLY" ]; then
    echo -n "Publish packages to S3? [y/N] "
    read response
    response=`echo $response | tr 'A-Z' 'a-z'`
fi

if [ "x$response" == "xy" -o -n "$NIGHTLY_BUILD" ]; then
    echo "Publishing packages to S3."
    if [ ! -e "$AWS_FILE" ]; then
        echo "$AWS_FILE does not exist -- aborting."
        cleanup_exit 1
    fi

    for filepath in `ls *.{$DEB_WANTED,$RPM_WANTED,$TAR_WANTED} 2> /dev/null`; do
        filename=`basename $filepath`

        if [ -n "$NIGHTLY_BUILD" ]; then
            # Replace the version string in the filename with "nightly".
            v=`full_version $VERSION $RC`
            v_underscored=`echo "$v" | tr - _`
            v_rpm=$VERSION-`rpm_release $RC`

            # It's ok to run each of these since only 1 will match, leaving
            # filename untouched otherwise.
            filename=`echo $filename | sed s/$v/nightly/`
            filename=`echo $filename | sed s/$v_underscored/nightly/`
            filename=`echo $filename | sed s/$v_rpm/nightly-1/`
        fi

        AWS_CONFIG_FILE=$AWS_FILE aws s3 cp $filepath s3://influxdb/$filename --acl public-read --region us-east-1
        if [ $? -ne 0 ]; then
            echo "Upload failed ($filename) -- aborting".
            cleanup_exit 1
        fi
        echo "$filename uploaded"
    done
else
    echo "Not publishing packages to S3."
fi

###########################################################################
# All done.

echo -e "\nPackaging process complete."
cleanup_exit 0
