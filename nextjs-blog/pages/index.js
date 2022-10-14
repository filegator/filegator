import Head from 'next/head'

export default function Home() {
  return (
    <>
  <Head>
    <title>CDN</title>
    <link rel="icon" href="https://res.cloudinary.com/weknow-creators/image/upload/v1665780570/images/favicon_ghlbjr.svg" />
    </Head>
  {/* <title>CodePen - 404 page</title> */}
  <link
    rel="stylesheet"
    href="https://cdnjs.cloudflare.com/ajax/libs/twitter-bootstrap/3.3.7/css/bootstrap.min.css"
  />
  <link rel="stylesheet" href="https://fonts.googleapis.com/css?family=Arvo" />
  {/* <link rel="stylesheet" href="./style.css" /> */}
  <section className="page_404">
    <div className="container">
      <div className="row">
        <div className="col-sm-12 ">
          <div className="col-sm-10 col-sm-offset-1  text-center">
            <div className="four_zero_four_bg">
              {/* <h1 className="text-center ">Oops</h1> */}
            </div>
            <div className="contant_box_404">
              <h3 className="h2">Go back</h3>
              <p>This page has nothing</p>
              <a href="https://mikeowino.com" target="_blank" className="link_404">
                Go to Home
              </a>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
  <style jsx>{`
  .page_404{ padding:40px 0; background:#fff; font-family: 'Arvo', serif;
}

.page_404  img{ width:100%;}

.four_zero_four_bg{
 
 background-image: url(https://res.cloudinary.com/weknow-creators/image/upload/v1665780420/images/dribbble_1_zxdjpu.gif);
    height: 400px;
    background-position: center;
 }
 
 
 .four_zero_four_bg h1{
 font-size:80px;
 }
 
  .four_zero_four_bg h3{
			 font-size:80px;
			 }
			 
			 .link_404{			 
	color: #fff!important;
    padding: 10px 20px;
    background: #39ac31;
    margin: 20px 0;
    display: inline-block;}
	.contant_box_404{ margin-top:-50px;}
      `}</style>
</>
  )
}
