DocRepo=$1
MainRepo=$(pwd)
echo $DocRepo $MainRepo

cp -r $MainRepo/docs/* $DocRepo
cd $DocRepo
git status
git add -A
git commit -m "syncing docs from main repo"
git push origin master